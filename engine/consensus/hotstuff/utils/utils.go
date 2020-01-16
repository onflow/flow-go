package utils

import (
	"github.com/dapperlabs/flow-go/engine/consensus/hotstuff/types"
	"reflect"
)

func EnsureNotNil(x interface{}, structName string) error {
	//some minor suggestions on EnsureNotNil, refer to https://medium.com/@mangatmodi/go-check-nil-interface-the-right-way-d142776edef1
	if x == nil {
		error :=  types.HotStuffConfigurationError(structName + " cannot be nil")
		return error
	}
	// ValueOf().IsNil() cannot be called on struct value
	// so first check if interface is kind pointer, else it is of course not nil
	switch reflect.TypeOf(x).Kind() {
	case reflect.Ptr, reflect.Map, reflect.Array, reflect.Chan, reflect.Slice:
		if reflect.ValueOf(x).IsNil() {
			panic(structName + " cannot be nil")
		}
	}
}
