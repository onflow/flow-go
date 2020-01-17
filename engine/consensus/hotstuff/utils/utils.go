package utils

import (
	"reflect"
)

// IsNil returns true if and only if interface implementation is nil.
// Uses reflection, i.e. implementation is slow.
func IsNil(x interface{}) bool {
	// Implementation inspired by https://medium.com/@mangatmodi/go-check-nil-interface-the-right-way-d142776edef1
	if x == nil {
		return true
	}
	// ValueOf().IsNil() cannot be called on struct value
	// so first check if interface is kind pointer, else it is of course not nil
	switch reflect.TypeOf(x).Kind() {
	case reflect.Ptr, reflect.Map, reflect.Array, reflect.Chan, reflect.Slice:
		if reflect.ValueOf(x).IsNil() {
			return true
		}
	}
	return false
}
