// Code generated by mockery v2.43.2. DO NOT EDIT.

package mock

import mock "github.com/stretchr/testify/mock"

// Subscription is an autogenerated mock type for the Subscription type
type Subscription struct {
	mock.Mock
}

// Channel provides a mock function with given fields:
func (_m *Subscription) Channel() <-chan interface{} {
	ret := _m.Called()

	if len(ret) == 0 {
		panic("no return value specified for Channel")
	}

	var r0 <-chan interface{}
	if rf, ok := ret.Get(0).(func() <-chan interface{}); ok {
		r0 = rf()
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(<-chan interface{})
		}
	}

	return r0
}

// Err provides a mock function with given fields:
func (_m *Subscription) Err() error {
	ret := _m.Called()

	if len(ret) == 0 {
		panic("no return value specified for Err")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func() error); ok {
		r0 = rf()
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// ID provides a mock function with given fields:
func (_m *Subscription) ID() string {
	ret := _m.Called()

	if len(ret) == 0 {
		panic("no return value specified for ID")
	}

	var r0 string
	if rf, ok := ret.Get(0).(func() string); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(string)
	}

	return r0
}

// NewSubscription creates a new instance of Subscription. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewSubscription(t interface {
	mock.TestingT
	Cleanup(func())
}) *Subscription {
	mock := &Subscription{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
