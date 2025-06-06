// Code generated by mockery v2.53.3. DO NOT EDIT.

package mock

import (
	flow "github.com/onflow/flow-go/model/flow"

	mock "github.com/stretchr/testify/mock"
)

// Vertex is an autogenerated mock type for the Vertex type
type Vertex struct {
	mock.Mock
}

// Level provides a mock function with no fields
func (_m *Vertex) Level() uint64 {
	ret := _m.Called()

	if len(ret) == 0 {
		panic("no return value specified for Level")
	}

	var r0 uint64
	if rf, ok := ret.Get(0).(func() uint64); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(uint64)
	}

	return r0
}

// Parent provides a mock function with no fields
func (_m *Vertex) Parent() (flow.Identifier, uint64) {
	ret := _m.Called()

	if len(ret) == 0 {
		panic("no return value specified for Parent")
	}

	var r0 flow.Identifier
	var r1 uint64
	if rf, ok := ret.Get(0).(func() (flow.Identifier, uint64)); ok {
		return rf()
	}
	if rf, ok := ret.Get(0).(func() flow.Identifier); ok {
		r0 = rf()
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(flow.Identifier)
		}
	}

	if rf, ok := ret.Get(1).(func() uint64); ok {
		r1 = rf()
	} else {
		r1 = ret.Get(1).(uint64)
	}

	return r0, r1
}

// VertexID provides a mock function with no fields
func (_m *Vertex) VertexID() flow.Identifier {
	ret := _m.Called()

	if len(ret) == 0 {
		panic("no return value specified for VertexID")
	}

	var r0 flow.Identifier
	if rf, ok := ret.Get(0).(func() flow.Identifier); ok {
		r0 = rf()
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(flow.Identifier)
		}
	}

	return r0
}

// NewVertex creates a new instance of Vertex. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewVertex(t interface {
	mock.TestingT
	Cleanup(func())
}) *Vertex {
	mock := &Vertex{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
