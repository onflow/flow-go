// Code generated by mockery v2.13.0. DO NOT EDIT.

package mockinsecure

import (
	insecure "github.com/onflow/flow-go/insecure"
	flow "github.com/onflow/flow-go/model/flow"

	mock "github.com/stretchr/testify/mock"

	network "github.com/onflow/flow-go/network"
)

// EgressController is an autogenerated mock type for the EgressController type
type EgressController struct {
	mock.Mock
}

// EngineClosingChannel provides a mock function with given fields: _a0
func (_m *EgressController) EngineClosingChannel(_a0 network.Channel) error {
	ret := _m.Called(_a0)

	var r0 error
	if rf, ok := ret.Get(0).(func(network.Channel) error); ok {
		r0 = rf(_a0)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// HandleOutgoingEvent provides a mock function with given fields: _a0, _a1, _a2, _a3, _a4
func (_m *EgressController) HandleOutgoingEvent(_a0 interface{}, _a1 network.Channel, _a2 insecure.Protocol, _a3 uint32, _a4 ...flow.Identifier) error {
	_va := make([]interface{}, len(_a4))
	for _i := range _a4 {
		_va[_i] = _a4[_i]
	}
	var _ca []interface{}
	_ca = append(_ca, _a0, _a1, _a2, _a3)
	_ca = append(_ca, _va...)
	ret := _m.Called(_ca...)

	var r0 error
	if rf, ok := ret.Get(0).(func(interface{}, network.Channel, insecure.Protocol, uint32, ...flow.Identifier) error); ok {
		r0 = rf(_a0, _a1, _a2, _a3, _a4...)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

type NewEgressControllerT interface {
	mock.TestingT
	Cleanup(func())
}

// NewEgressController creates a new instance of EgressController. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
func NewEgressController(t NewEgressControllerT) *EgressController {
	mock := &EgressController{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
