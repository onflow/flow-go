// Code generated by mockery v2.53.3. DO NOT EDIT.

package mock

import (
	context "context"

	mock "github.com/stretchr/testify/mock"
)

// Core is an autogenerated mock type for the Core type
type Core struct {
	mock.Mock
}

// Abandon provides a mock function with no fields
func (_m *Core) Abandon() error {
	ret := _m.Called()

	if len(ret) == 0 {
		panic("no return value specified for Abandon")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func() error); ok {
		r0 = rf()
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// Download provides a mock function with given fields: ctx
func (_m *Core) Download(ctx context.Context) error {
	ret := _m.Called(ctx)

	if len(ret) == 0 {
		panic("no return value specified for Download")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context) error); ok {
		r0 = rf(ctx)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// Index provides a mock function with no fields
func (_m *Core) Index() error {
	ret := _m.Called()

	if len(ret) == 0 {
		panic("no return value specified for Index")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func() error); ok {
		r0 = rf()
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// Persist provides a mock function with no fields
func (_m *Core) Persist() error {
	ret := _m.Called()

	if len(ret) == 0 {
		panic("no return value specified for Persist")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func() error); ok {
		r0 = rf()
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// NewCore creates a new instance of Core. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewCore(t interface {
	mock.TestingT
	Cleanup(func())
}) *Core {
	mock := &Core{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
