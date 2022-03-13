package diingo

import (
	"context"
	"fmt"
	"golang.org/x/sync/errgroup"
	"reflect"
	"sync"
)

type dNode struct {
	dType           reflect.Type
	dependencyTypes map[reflect.Type]struct{}
	dependencies    map[reflect.Type]*dependency
	constructor     reflect.Value
	mutex           sync.Mutex
	value           reflect.Value
}

type dependency struct {
	nodes          []*dNode
	dependencyType reflect.Type
}

func (d *dependency) values(ctx context.Context) (reflect.Value, error) {
	if len(d.nodes) == 0 {
		return reflect.MakeSlice(d.dependencyType, 0, 0), nil
	}
	value := reflect.MakeSlice(d.dependencyType, len(d.nodes), len(d.nodes))
	subGroup, _ := errgroup.WithContext(ctx)
	for j, node := range d.nodes {
		index := j
		dNode := node
		subGroup.Go(func() error {
			nodeValue, err := dNode.Value()
			if err != nil {
				return err
			}
			if !nodeValue.IsValid() {
				return fmt.Errorf(UnableToResolveDependency, d.dependencyType)
			}
			value.Index(index).Set(nodeValue)
			return nil
		})
	}
	if err := subGroup.Wait(); err != nil {
		return reflect.Value{}, err
	}
	return value, nil
}

func newFunctionNode(constructor reflect.Value) *dNode {
	constructorType := constructor.Type()
	nodeType := constructorType.Out(0)
	dependencyTypes := make(map[reflect.Type]struct{})
	for i := 0; i < constructorType.NumIn(); i++ {
		dependencyTypes[constructorType.In(i)] = struct{}{}
	}
	return &dNode{
		dType:           nodeType,
		dependencyTypes: dependencyTypes,
		dependencies:    map[reflect.Type]*dependency{},
		constructor:     constructor,
		mutex:           sync.Mutex{},
	}
}

func newValueNode(value reflect.Value) *dNode {
	return &dNode{
		dType:           value.Type(),
		dependencyTypes: map[reflect.Type]struct{}{},
		dependencies:    map[reflect.Type]*dependency{},
		constructor:     reflect.Value{},
		mutex:           sync.Mutex{},
		value:           value,
	}
}

func newRootNode(nodeType reflect.Type) *dNode {
	return newFunctionNode(makeConstructor(nodeType))
}

func (n *dNode) Value() (reflect.Value, error) {
	n.mutex.Lock()
	value, err := func() (reflect.Value, error) {
		if n.value.IsValid() {
			return n.value, nil
		}
		numIn := n.constructor.Type().NumIn()
		arguments := make([]reflect.Value, numIn)
		mainGroup, ctx := errgroup.WithContext(context.Background())
		for i := 0; i < numIn; i++ {
			index := i
			mainGroup.Go(func() error {
				var err error
				argType := n.constructor.Type().In(index)
				value, err := func() (reflect.Value, error) {
					dependency, ok := n.dependencies[argType]
					if !ok {
						if argType.Kind() != reflect.Slice {
							return reflect.Value{}, fmt.Errorf(MissingDependency, argType)
						}
						return reflect.MakeSlice(argType, 0, 0), nil
					}
					if argType.Kind() != reflect.Slice {
						nodes := dependency.nodes
						if len(nodes) == 0 {
							return reflect.Value{}, fmt.Errorf(UnableToResolveDependency, argType)
						}
						return nodes[0].Value()
					}
					return dependency.values(ctx)
				}()
				if err != nil {
					return fmt.Errorf(UnableToResolveDependencyCause, n.dType, err)
				}
				if !value.IsValid() {
					return fmt.Errorf(UnableToResolveDependency, n.dType)
				}
				arguments[index] = value
				return nil
			})
		}
		if err := mainGroup.Wait(); err != nil {
			return reflect.Value{}, err
		}
		results := n.constructor.Call(arguments)
		if len(results) > 1 {
			err := results[1]
			if !err.IsNil() {
				return reflect.Value{}, err.Interface().(error)
			}
		}
		n.value = results[0]
		if !n.value.IsValid() {
			return reflect.Value{}, fmt.Errorf(UnableToResolveDependency, n.dType)
		}
		return n.value, nil
	}()
	n.mutex.Unlock()
	return value, err
}

func (n *dNode) confirmDependencyWith(dependant *dNode) {
	for dependencyType := range dependant.dependencyTypes {
		if usableAs(n.dType, dependencyType) {
			dependant.dependencies[dependencyType] = &dependency{nodes: []*dNode{n}, dependencyType: dependencyType}
		} else if dependencyType.Kind() == reflect.Slice && usableAs(n.dType, dependencyType.Elem()) {
			if d, ok := dependant.dependencies[dependencyType]; ok {
				d.nodes = append(d.nodes, n)
			} else {
				dependant.dependencies[dependencyType] = &dependency{nodes: []*dNode{n}, dependencyType: dependencyType}
			}
		}
	}
}

func makeConstructor(returnType reflect.Type) reflect.Value {
	arguments := make([]reflect.Type, 0, returnType.NumField())
	for i := 0; i < returnType.NumField(); i++ {
		arguments = append(arguments, returnType.Field(i).Type)
	}
	return reflect.MakeFunc(reflect.FuncOf(arguments, []reflect.Type{returnType}, false), func(args []reflect.Value) (results []reflect.Value) {
		returnValue := func() reflect.Value {
			if returnType.Kind() == reflect.Ptr || returnType.Kind() == reflect.Interface {
				return reflect.New(returnType.Elem())
			}
			return reflect.New(returnType)
		}().Elem()
		for i, arg := range args {
			returnValue.Field(i).Set(arg)
		}
		return []reflect.Value{returnValue}
	})
}

func usableAs(typ1 reflect.Type, typ2 reflect.Type) bool {
	return typ1.AssignableTo(typ2) || typ2.Kind() == reflect.Interface && typ1.Implements(typ2)
}

func NewApplicationContext(applicationContext interface{}, providers ...interface{}) (interface{}, error) {
	rootType := reflect.Indirect(reflect.ValueOf(applicationContext)).Type()
	rootNode := newRootNode(rootType)
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
		return nil, err
	}
	if !value.IsValid() {
		return nil, fmt.Errorf("failed to resolve dependencies")
	}
	ptr := reflect.New(rootType)
	ptr.Elem().Set(value)
	return ptr.Interface(), err
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
