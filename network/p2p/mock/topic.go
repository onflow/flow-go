// Code generated by mockery v2.13.1. DO NOT EDIT.

package mockp2p

import (
	context "context"

	p2p "github.com/onflow/flow-go/network/p2p"
	mock "github.com/stretchr/testify/mock"
)

// Topic is an autogenerated mock type for the Topic type
type Topic struct {
	mock.Mock
}

// Close provides a mock function with given fields:
func (_m *Topic) Close() error {
	ret := _m.Called()

	var r0 error
	if rf, ok := ret.Get(0).(func() error); ok {
		r0 = rf()
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// Publish provides a mock function with given fields: _a0, _a1
func (_m *Topic) Publish(_a0 context.Context, _a1 []byte) error {
	ret := _m.Called(_a0, _a1)

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, []byte) error); ok {
		r0 = rf(_a0, _a1)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// String provides a mock function with given fields:
func (_m *Topic) String() string {
	ret := _m.Called()

	var r0 string
	if rf, ok := ret.Get(0).(func() string); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(string)
	}

	return r0
}

// Subscribe provides a mock function with given fields:
func (_m *Topic) Subscribe() (p2p.Subscription, error) {
	ret := _m.Called()

	var r0 p2p.Subscription
	if rf, ok := ret.Get(0).(func() p2p.Subscription); ok {
		r0 = rf()
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(p2p.Subscription)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func() error); ok {
		r1 = rf()
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

type mockConstructorTestingTNewTopic interface {
	mock.TestingT
	Cleanup(func())
}

// NewTopic creates a new instance of Topic. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
func NewTopic(t mockConstructorTestingTNewTopic) *Topic {
	mock := &Topic{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
