package utils

import "reflect"

// IsNil checks if the input is nil, and works for any type including interfaces.
// This method uses reflection. Avoid use in performance-critical code.
func IsNil(v interface{}) bool {
	if v == nil {
		return true
	}
	rv := reflect.ValueOf(v)
	switch rv.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Ptr, reflect.Slice:
		return rv.IsNil()
	default:
		return false
	}
}

// NotNil verifies that the input is not nil and returns it. It panics if the input is nil.
// This is useful when checking dependencies are initialized during bootstrap.
// This method uses reflection. Avoid use in performance-critical code.
func NotNil[T any](v T) T {
	if IsNil(v) {
		panic("value is nil")
	}
	return v
}
