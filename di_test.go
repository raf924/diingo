package diingo

import (
	"fmt"
	"reflect"
	"strings"
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

	ctx, err := NewApplicationContext(&context{}, rootConstructor, dependencyConstructor1, dependencyConstructor2)
	if err != nil {
		t.Fatal(err)
	}

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

	ctx, err := NewApplicationContext(&context{}, root1, root2)
	if err != nil {
		t.Fatal(err)
	}

	roots := ctx.(*context).Roots

	if len(roots) != 2 || roots[0] != "root1" || roots[1] != "root2" {
		t.Fatal("wrong", roots)
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
	_, err := NewApplicationContext(&context{}, root1)
	expectedError := fmt.Errorf(UnableToResolveDependencyCause, reflect.TypeOf(context{}), fmt.Errorf(MissingDependency, reflect.TypeOf(type2(""))))
	if err.(error).Error() != expectedError.Error() {
		t.Fatal("expected", expectedError, "got", err)
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

	ctx, err := NewApplicationContext(&context{}, func() *testType {
		return &testType{}
	})

	if err != nil {
		t.Fatal(err)
	}

	tests := ctx.(*context).Tests
	if len(tests) != 1 {
		t.Fatal("wrong", tests)
	}
}

func TestNewApplicationContextWithCyclicDependency(t *testing.T) {
	type type1 string
	type type2 string

	type c struct {
		t type1
	}

	_, err := NewApplicationContext(c{}, func(t type1) type2 {
		return ""
	}, func(t type2) type1 {
		return ""
	})
	expectedError := fmt.Errorf(CyclicDependency, reflect.TypeOf(type1("")))
	if !strings.Contains(err.(error).Error(), expectedError.Error()) {
		t.Fatal("expected", expectedError, "got", err)
	}
}

func TestNewApplicationContextWithPointer(t *testing.T) {
	type type1 string
	type c struct {
		T type1
	}

	ctx, err := NewApplicationContext(&c{}, func() *type1 {
		var s = "hullo"
		return (*type1)(&s)
	})
	if err != nil {
		t.Fatal("unexpected error", err)
	}
	if ctx.(*c).T != "hullo" {
		t.Fatal("expected hullo got", ctx.(*c).T)
	}
}

func TestNewApplicationContextWithNonPointer(t *testing.T) {
	type type1 string
	type c struct {
		T *type1
	}

	ctx, err := NewApplicationContext(&c{}, func() type1 {
		return "hullo"
	})
	if err != nil {
		t.Fatal("unexpected error", err)
	}
	if *ctx.(*c).T != "hullo" {
		t.Fatal("expected hullo got", ctx.(*c).T)
	}
}
