// Code generated by mockery v2.13.1. DO NOT EDIT.

package mock

import (
	flow "github.com/onflow/flow-go/model/flow"
	irrecoverable "github.com/onflow/flow-go/module/irrecoverable"

	mock "github.com/stretchr/testify/mock"
)

// HotStuff is an autogenerated mock type for the HotStuff type
type HotStuff struct {
	mock.Mock
}

// Done provides a mock function with given fields:
func (_m *HotStuff) Done() <-chan struct{} {
	ret := _m.Called()

	var r0 <-chan struct{}
	if rf, ok := ret.Get(0).(func() <-chan struct{}); ok {
		r0 = rf()
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(<-chan struct{})
		}
	}

	return r0
}

// Ready provides a mock function with given fields:
func (_m *HotStuff) Ready() <-chan struct{} {
	ret := _m.Called()

	var r0 <-chan struct{}
	if rf, ok := ret.Get(0).(func() <-chan struct{}); ok {
		r0 = rf()
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(<-chan struct{})
		}
	}

	return r0
}

// Start provides a mock function with given fields: _a0
func (_m *HotStuff) Start(_a0 irrecoverable.SignalerContext) {
	_m.Called(_a0)
}

// SubmitProposal provides a mock function with given fields: proposal, parentView
func (_m *HotStuff) SubmitProposal(proposal *flow.Header, parentView uint64) <-chan struct{} {
	ret := _m.Called(proposal, parentView)

	var r0 <-chan struct{}
	if rf, ok := ret.Get(0).(func(*flow.Header, uint64) <-chan struct{}); ok {
		r0 = rf(proposal, parentView)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(<-chan struct{})
		}
	}

	return r0
}

type mockConstructorTestingTNewHotStuff interface {
	mock.TestingT
	Cleanup(func())
}

// NewHotStuff creates a new instance of HotStuff. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
func NewHotStuff(t mockConstructorTestingTNewHotStuff) *HotStuff {
	mock := &HotStuff{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
