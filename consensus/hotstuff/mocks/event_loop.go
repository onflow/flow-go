// Code generated by mockery v2.21.4. DO NOT EDIT.

package mocks

import (
	flow "github.com/onflow/flow-go/model/flow"

	irrecoverable "github.com/onflow/flow-go/module/irrecoverable"

	mock "github.com/stretchr/testify/mock"

	model "github.com/onflow/flow-go/consensus/hotstuff/model"
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

// OnNewQcDiscovered provides a mock function with given fields: certificate
func (_m *EventLoop) OnNewQcDiscovered(certificate *flow.QuorumCertificate) {
	_m.Called(certificate)
}

// OnNewTcDiscovered provides a mock function with given fields: certificate
func (_m *EventLoop) OnNewTcDiscovered(certificate *flow.TimeoutCertificate) {
	_m.Called(certificate)
}

// OnPartialTcCreated provides a mock function with given fields: view, newestQC, lastViewTC
func (_m *EventLoop) OnPartialTcCreated(view uint64, newestQC *flow.QuorumCertificate, lastViewTC *flow.TimeoutCertificate) {
	_m.Called(view, newestQC, lastViewTC)
}

// OnQcConstructedFromVotes provides a mock function with given fields: _a0
func (_m *EventLoop) OnQcConstructedFromVotes(_a0 *flow.QuorumCertificate) {
	_m.Called(_a0)
}

// OnTcConstructedFromTimeouts provides a mock function with given fields: certificate
func (_m *EventLoop) OnTcConstructedFromTimeouts(certificate *flow.TimeoutCertificate) {
	_m.Called(certificate)
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

// SubmitProposal provides a mock function with given fields: proposal
func (_m *EventLoop) SubmitProposal(proposal *model.Proposal) {
	_m.Called(proposal)
}

type mockConstructorTestingTNewEventLoop interface {
	mock.TestingT
	Cleanup(func())
}

// NewEventLoop creates a new instance of EventLoop. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
func NewEventLoop(t mockConstructorTestingTNewEventLoop) *EventLoop {
	mock := &EventLoop{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
