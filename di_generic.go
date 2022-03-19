//go:build go1.18

package diingo

import (
	"fmt"
	"reflect"
)

type ProviderFuncWithError[T any] func(dependencies ...any) (T, error)

type ProviderFunc[T any] func(dependencies ...any) T

type Provider[T any] interface {
	ProviderFunc[any] | ProviderFuncWithError[any]
}

type provider interface {
	provide(dependencies ...any) (any, error)
}

var _ provider = ProviderFunc[any](nil)
var _ provider = ProviderFuncWithError[any](nil)

func (p ProviderFunc[T]) provide(dependencies ...any) (T, error) {
	return p(dependencies), nil
}

func (p ProviderFuncWithError[T]) provide(dependencies ...any) (T, error) {
	return p(dependencies)
}

func LoadDependencies[T any](obj *T, providers ...any) error {
	rootType := reflect.TypeOf(obj)
	rootNode := newFunctionNode(createConstructor(obj, rootType))
	dependencyNodes := createDependencyNodes(providers...)
	for _, node := range dependencyNodes {
		node.confirmDependencyWith(rootNode)
		for _, constructorNode := range dependencyNodes {
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
	return nil
}

func createDependencyNodes[P Provider[any] | any](providers ...P) []*dNode {
	constructorNodes := make([]*dNode, 0, len(providers))
	for _, provider := range providers {
		providerValue := reflect.ValueOf(provider)
		constructorNodes = append(constructorNodes, newValueNode(providerValue))
		if providerValue.Kind() == reflect.Func {
			constructorNodes = append(constructorNodes, newFunctionNode(providerValue))
		}
	}
	return constructorNodes
}

func createConstructor[T any](obj *T, returnType reflect.Type) reflect.Value {
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
