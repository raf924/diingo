//go:build !go1.18

package diingo

import (
	"fmt"
	"reflect"
)

func LoadDependencies(obj interface{}, providers ...interface{}) error {
	rootType := reflect.TypeOf(obj)
	rootNode := newFunctionNode(createConstructor(obj, rootType))
	constructorNodes := createProviderNodes(providers)
	for _, node := range constructorNodes {
		node.confirmDependencyWith(rootNode)
		for _, constructorNode := range constructorNodes {
			if constructorNode == node {
				continue
			}
			node.confirmDependencyWith(constructorNode)
		}
	}
	value, err := rootNode.Value()
	if err != nil {
		return err
	}
	if !value.IsValid() {
		return fmt.Errorf("failed to resolve dependencies")
	}
	return err
}

func createConstructor(obj interface{}, returnType reflect.Type) reflect.Value {
	elemType := returnType.Elem()
	arguments := make([]reflect.Type, 0, elemType.NumField())
	for i := 0; i < elemType.NumField(); i++ {
		arguments = append(arguments, elemType.Field(i).Type)
	}
	return reflect.MakeFunc(reflect.FuncOf(arguments, []reflect.Type{returnType}, false), func(args []reflect.Value) (results []reflect.Value) {
		returnValue := reflect.ValueOf(obj)
		for i, arg := range args {
			returnValue.Elem().Field(i).Set(arg)
		}
		return []reflect.Value{returnValue}
	})
}

func createProviderNodes(constructors []interface{}) []*dNode {
	constructorNodes := make([]*dNode, 0, len(constructors))
	for _, constructor := range constructors {
		constructorValue := reflect.ValueOf(constructor)
		constructorNodes = append(constructorNodes, newValueNode(constructorValue))
		if constructorValue.Kind() == reflect.Func {
			constructorNodes = append(constructorNodes, newFunctionNode(constructorValue))
		}
	}
	return constructorNodes
}
