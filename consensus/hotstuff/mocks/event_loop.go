// Code generated by mockery v1.0.0. DO NOT EDIT.

package mocks

import (
	flow "github.com/onflow/flow-go/model/flow"

	irrecoverable "github.com/onflow/flow-go/module/irrecoverable"

	mock "github.com/stretchr/testify/mock"
)

// EventLoop is an autogenerated mock type for the EventLoop type
type EventLoop struct {
	mock.Mock
}

// Done provides a mock function with given fields:
func (_m *EventLoop) Done() <-chan struct{} {
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
func (_m *EventLoop) Ready() <-chan struct{} {
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
func (_m *EventLoop) Start(_a0 irrecoverable.SignalerContext) {
	_m.Called(_a0)
}

// SubmitProposal provides a mock function with given fields: proposal, parentView
func (_m *EventLoop) SubmitProposal(proposal *flow.Header, parentView uint64) {
	_m.Called(proposal, parentView)
}

// SubmitTrustedQC provides a mock function with given fields: qc
func (_m *EventLoop) SubmitTrustedQC(qc *flow.QuorumCertificate) {
	_m.Called(qc)
}
