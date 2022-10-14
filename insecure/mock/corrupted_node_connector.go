// Code generated by mockery v2.13.1. DO NOT EDIT.

package mockinsecure

import (
	insecure "github.com/onflow/flow-go/insecure"
	flow "github.com/onflow/flow-go/model/flow"

	irrecoverable "github.com/onflow/flow-go/module/irrecoverable"

	mock "github.com/stretchr/testify/mock"
)

// CorruptedNodeConnector is an autogenerated mock type for the CorruptedNodeConnector type
type CorruptedNodeConnector struct {
	mock.Mock
}

// Connect provides a mock function with given fields: _a0, _a1
func (_m *CorruptedNodeConnector) Connect(_a0 irrecoverable.SignalerContext, _a1 flow.Identifier) (insecure.CorruptNodeConnection, error) {
	ret := _m.Called(_a0, _a1)

	var r0 insecure.CorruptNodeConnection
	if rf, ok := ret.Get(0).(func(irrecoverable.SignalerContext, flow.Identifier) insecure.CorruptNodeConnection); ok {
		r0 = rf(_a0, _a1)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(insecure.CorruptNodeConnection)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(irrecoverable.SignalerContext, flow.Identifier) error); ok {
		r1 = rf(_a0, _a1)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// WithIncomingMessageHandler provides a mock function with given fields: _a0
func (_m *CorruptedNodeConnector) WithIncomingMessageHandler(_a0 func(*insecure.Message)) {
	_m.Called(_a0)
}

type mockConstructorTestingTNewCorruptedNodeConnector interface {
	mock.TestingT
	Cleanup(func())
}

// NewCorruptedNodeConnector creates a new instance of CorruptedNodeConnector. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
func NewCorruptedNodeConnector(t mockConstructorTestingTNewCorruptedNodeConnector) *CorruptedNodeConnector {
	mock := &CorruptedNodeConnector{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
