package diingo

import (
	"fmt"
	"testing"
)

func TestLoadDependencies(t *testing.T) {
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

	var c context

	err := LoadDependencies(&c, rootConstructor, dependencyConstructor1, dependencyConstructor2)
	if err != nil {
		t.Fatal(err)
	}

	expectedValue := "root,dependency1,dependency2,dependency1"
	if c.Root != rootType(expectedValue) {
		t.Fatal("expected", expectedValue, "got", c.Root)
	}
}
