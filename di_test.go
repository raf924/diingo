package di

import (
	"fmt"
	"reflect"
	"testing"
)

func TestNewApplicationContext(t *testing.T) {
	type rootType string
	type context struct {
		Root rootType
	}
	type dependencyType1 string
	type dependencyType2 string
	var rootConstructor = func(type1 dependencyType1, type2 dependencyType2) rootType {
		return rootType(fmt.Sprintf("root,%s,%s", type1, type2))
	}
	var dependencyConstructor1 = func() dependencyType1 {
		return "dependency1"
	}
	var dependencyConstructor2 = func(type1 dependencyType1) dependencyType2 {
		return dependencyType2(fmt.Sprintf("dependency2,%s", type1))
	}

	ctx := NewApplicationContext(&context{}, rootConstructor, dependencyConstructor1, dependencyConstructor2)

	expectedValue := "root,dependency1,dependency2,dependency1"
	if ctx.(*context).Root != rootType(expectedValue) {
		t.Fatal("expected", expectedValue, "got", ctx.(context).Root)
	}
}

func TestNewApplicationContextWithArray(t *testing.T) {
	type rootType string
	type context struct {
		Roots []rootType
	}
	var root1 = func() rootType { return "root1" }
	var root2 = func() rootType { return "root2" }

	ctx := NewApplicationContext(&context{}, root1, root2).(*context)

	if len(ctx.Roots) != 2 || ctx.Roots[0] != "root1" || ctx.Roots[1] != "root2" {
		t.Fatal("wrong", ctx.Roots)
	}
}

func TestNewApplicationContextMissingDependency(t *testing.T) {
	type type1 string
	type type2 string
	type context struct {
		Root1 type1
		Root2 type2
	}
	var root1 = func() type1 { return "root1" }
	defer func() {
		err := recover()
		expectedError := fmt.Errorf(UnableToResolveDependencyCause, reflect.TypeOf(context{}), fmt.Errorf(MissingDependency, reflect.TypeOf(type2(""))))
		if err.(error).Error() != expectedError.Error() {
			t.Fatal("expected", expectedError, "got", err)
		}
	}()
	ctx := NewApplicationContext(&context{}, root1)
	if ctx != nil {
		t.Fatal("should have failed")
	}
}

type testInterface interface {
	do()
}

type testType struct {
}

func (t *testType) do() {
	println("do")
}

func TestNewApplicationContextWithInterfaceArray(t *testing.T) {
	type context struct {
		Tests []testInterface
	}

	ctx := NewApplicationContext(&context{}, func() *testType {
		return &testType{}
	})

	tests := ctx.(*context).Tests
	if len(tests) != 1 {
		t.Fatal("wrong", tests)
	}
}
