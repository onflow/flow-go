// Code generated by mockery v2.53.3. DO NOT EDIT.

package mock

import (
	flow "github.com/onflow/flow-go/model/flow"
	mock "github.com/stretchr/testify/mock"
)

// Snapshot is an autogenerated mock type for the Snapshot type
type Snapshot struct {
	mock.Mock
}

// Collection provides a mock function with no fields
func (_m *Snapshot) Collection() (*flow.Collection, error) {
	ret := _m.Called()

	if len(ret) == 0 {
		panic("no return value specified for Collection")
	}

	var r0 *flow.Collection
	var r1 error
	if rf, ok := ret.Get(0).(func() (*flow.Collection, error)); ok {
		return rf()
	}
	if rf, ok := ret.Get(0).(func() *flow.Collection); ok {
		r0 = rf()
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*flow.Collection)
		}
	}

	if rf, ok := ret.Get(1).(func() error); ok {
		r1 = rf()
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// Head provides a mock function with no fields
func (_m *Snapshot) Head() (*flow.Header, error) {
	ret := _m.Called()

	if len(ret) == 0 {
		panic("no return value specified for Head")
	}

	var r0 *flow.Header
	var r1 error
	if rf, ok := ret.Get(0).(func() (*flow.Header, error)); ok {
		return rf()
	}
	if rf, ok := ret.Get(0).(func() *flow.Header); ok {
		r0 = rf()
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*flow.Header)
		}
	}

	if rf, ok := ret.Get(1).(func() error); ok {
		r1 = rf()
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// Pending provides a mock function with no fields
func (_m *Snapshot) Pending() ([]flow.Identifier, error) {
	ret := _m.Called()

	if len(ret) == 0 {
		panic("no return value specified for Pending")
	}

	var r0 []flow.Identifier
	var r1 error
	if rf, ok := ret.Get(0).(func() ([]flow.Identifier, error)); ok {
		return rf()
	}
	if rf, ok := ret.Get(0).(func() []flow.Identifier); ok {
		r0 = rf()
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]flow.Identifier)
		}
	}

	if rf, ok := ret.Get(1).(func() error); ok {
		r1 = rf()
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// NewSnapshot creates a new instance of Snapshot. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewSnapshot(t interface {
	mock.TestingT
	Cleanup(func())
}) *Snapshot {
	mock := &Snapshot{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
