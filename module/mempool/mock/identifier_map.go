// Code generated by mockery v2.43.2. DO NOT EDIT.

package mempool

import (
	flow "github.com/onflow/flow-go/model/flow"

	mock "github.com/stretchr/testify/mock"
)

// IdentifierMap is an autogenerated mock type for the IdentifierMap type
type IdentifierMap struct {
	mock.Mock
}

// Append provides a mock function with given fields: key, id
func (_m *IdentifierMap) Append(key flow.Identifier, id flow.Identifier) error {
	ret := _m.Called(key, id)

	if len(ret) == 0 {
		panic("no return value specified for Append")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(flow.Identifier, flow.Identifier) error); ok {
		r0 = rf(key, id)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// Get provides a mock function with given fields: key
func (_m *IdentifierMap) Get(key flow.Identifier) ([]flow.Identifier, bool) {
	ret := _m.Called(key)

	if len(ret) == 0 {
		panic("no return value specified for Get")
	}

	var r0 []flow.Identifier
	var r1 bool
	if rf, ok := ret.Get(0).(func(flow.Identifier) ([]flow.Identifier, bool)); ok {
		return rf(key)
	}
	if rf, ok := ret.Get(0).(func(flow.Identifier) []flow.Identifier); ok {
		r0 = rf(key)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]flow.Identifier)
		}
	}

	if rf, ok := ret.Get(1).(func(flow.Identifier) bool); ok {
		r1 = rf(key)
	} else {
		r1 = ret.Get(1).(bool)
	}

	return r0, r1
}

// Has provides a mock function with given fields: key
func (_m *IdentifierMap) Has(key flow.Identifier) bool {
	ret := _m.Called(key)

	if len(ret) == 0 {
		panic("no return value specified for Has")
	}

	var r0 bool
	if rf, ok := ret.Get(0).(func(flow.Identifier) bool); ok {
		r0 = rf(key)
	} else {
		r0 = ret.Get(0).(bool)
	}

	return r0
}

// Keys provides a mock function with given fields:
func (_m *IdentifierMap) Keys() ([]flow.Identifier, bool) {
	ret := _m.Called()

	if len(ret) == 0 {
		panic("no return value specified for Keys")
	}

	var r0 []flow.Identifier
	var r1 bool
	if rf, ok := ret.Get(0).(func() ([]flow.Identifier, bool)); ok {
		return rf()
	}
	if rf, ok := ret.Get(0).(func() []flow.Identifier); ok {
		r0 = rf()
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]flow.Identifier)
		}
	}

	if rf, ok := ret.Get(1).(func() bool); ok {
		r1 = rf()
	} else {
		r1 = ret.Get(1).(bool)
	}

	return r0, r1
}

// Remove provides a mock function with given fields: key
func (_m *IdentifierMap) Remove(key flow.Identifier) bool {
	ret := _m.Called(key)

	if len(ret) == 0 {
		panic("no return value specified for Remove")
	}

	var r0 bool
	if rf, ok := ret.Get(0).(func(flow.Identifier) bool); ok {
		r0 = rf(key)
	} else {
		r0 = ret.Get(0).(bool)
	}

	return r0
}

// RemoveIdFromKey provides a mock function with given fields: key, id
func (_m *IdentifierMap) RemoveIdFromKey(key flow.Identifier, id flow.Identifier) error {
	ret := _m.Called(key, id)

	if len(ret) == 0 {
		panic("no return value specified for RemoveIdFromKey")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(flow.Identifier, flow.Identifier) error); ok {
		r0 = rf(key, id)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// Size provides a mock function with given fields:
func (_m *IdentifierMap) Size() uint {
	ret := _m.Called()

	if len(ret) == 0 {
		panic("no return value specified for Size")
	}

	var r0 uint
	if rf, ok := ret.Get(0).(func() uint); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(uint)
	}

	return r0
}

// NewIdentifierMap creates a new instance of IdentifierMap. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewIdentifierMap(t interface {
	mock.TestingT
	Cleanup(func())
}) *IdentifierMap {
	mock := &IdentifierMap{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
