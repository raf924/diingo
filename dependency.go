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

func (d *dependency) values(ctx context.Context, dependencyChain ...*dNode) (reflect.Value, error) {
	if len(d.nodes) == 0 {
		return reflect.MakeSlice(d.dependencyType, 0, 0), nil
	}
	value := reflect.MakeSlice(d.dependencyType, len(d.nodes), len(d.nodes))
	subGroup, _ := errgroup.WithContext(ctx)
	for j, node := range d.nodes {
		index := j
		dNode := node
		subGroup.Go(func() error {
			nodeValue, err := dNode.Value(dependencyChain...)
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

func (n *dNode) Value(dependencyChain ...*dNode) (reflect.Value, error) {
	for _, node := range dependencyChain {
		if node == n {
			return reflect.Value{}, fmt.Errorf(CyclicDependency, n.dType.String())
		}
	}
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
						return nodes[0].Value(append(dependencyChain, n)...)
					}
					return dependency.values(ctx)
				}()
				if err != nil {
					return fmt.Errorf(UnableToResolveDependencyCause, n.dType, err)
				}
				if !value.IsValid() {
					return fmt.Errorf(UnableToResolveDependency, n.dType)
				}
				if value.Kind() == reflect.Ptr && value.Type() == reflect.PtrTo(argType) {
					value = value.Elem()
				} else if argType.Kind() == reflect.Ptr && argType == reflect.PtrTo(value.Type()) {
					newValue := reflect.New(value.Type())
					newValue.Elem().Set(value)
					value = newValue
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

func usableAs(typ1 reflect.Type, typ2 reflect.Type) bool {
	return typ1.AssignableTo(typ2) ||
		typ2.Kind() == reflect.Interface && typ1.Implements(typ2) ||
		typ1.Kind() == reflect.Ptr && usableAs(typ1.Elem(), typ2) ||
		typ2.Kind() == reflect.Ptr && usableAs(typ1, typ2.Elem())
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
