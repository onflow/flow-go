// Code generated by mockery v2.13.1. DO NOT EDIT.

package mocknetwork

import (
	flow "github.com/onflow/flow-go/model/flow"
	channels "github.com/onflow/flow-go/network/channels"

	mock "github.com/stretchr/testify/mock"

	peer "github.com/libp2p/go-libp2p-core/peer"
)

// Adapter is an autogenerated mock type for the Adapter type
type Adapter struct {
	mock.Mock
}

// MulticastOnChannel provides a mock function with given fields: _a0, _a1, _a2, _a3
func (_m *Adapter) MulticastOnChannel(_a0 channels.Channel, _a1 interface{}, _a2 uint, _a3 ...flow.Identifier) error {
	_va := make([]interface{}, len(_a3))
	for _i := range _a3 {
		_va[_i] = _a3[_i]
	}
	var _ca []interface{}
	_ca = append(_ca, _a0, _a1, _a2)
	_ca = append(_ca, _va...)
	ret := _m.Called(_ca...)

	var r0 error
	if rf, ok := ret.Get(0).(func(channels.Channel, interface{}, uint, ...flow.Identifier) error); ok {
		r0 = rf(_a0, _a1, _a2, _a3...)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// PublishOnChannel provides a mock function with given fields: _a0, _a1, _a2
func (_m *Adapter) PublishOnChannel(_a0 channels.Channel, _a1 interface{}, _a2 ...flow.Identifier) error {
	_va := make([]interface{}, len(_a2))
	for _i := range _a2 {
		_va[_i] = _a2[_i]
	}
	var _ca []interface{}
	_ca = append(_ca, _a0, _a1)
	_ca = append(_ca, _va...)
	ret := _m.Called(_ca...)

	var r0 error
	if rf, ok := ret.Get(0).(func(channels.Channel, interface{}, ...flow.Identifier) error); ok {
		r0 = rf(_a0, _a1, _a2...)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// SendReputationFeedback provides a mock function with given fields: id, messageID, topic, feedBack
func (_m *Adapter) SendReputationFeedback(id peer.ID, messageID string, topic channels.Topic, feedBack bool) {
	_m.Called(id, messageID, topic, feedBack)
}

// UnRegisterChannel provides a mock function with given fields: channel
func (_m *Adapter) UnRegisterChannel(channel channels.Channel) error {
	ret := _m.Called(channel)

	var r0 error
	if rf, ok := ret.Get(0).(func(channels.Channel) error); ok {
		r0 = rf(channel)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// UnicastOnChannel provides a mock function with given fields: _a0, _a1, _a2
func (_m *Adapter) UnicastOnChannel(_a0 channels.Channel, _a1 interface{}, _a2 flow.Identifier) error {
	ret := _m.Called(_a0, _a1, _a2)

	var r0 error
	if rf, ok := ret.Get(0).(func(channels.Channel, interface{}, flow.Identifier) error); ok {
		r0 = rf(_a0, _a1, _a2)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

type mockConstructorTestingTNewAdapter interface {
	mock.TestingT
	Cleanup(func())
}

// NewAdapter creates a new instance of Adapter. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
func NewAdapter(t mockConstructorTestingTNewAdapter) *Adapter {
	mock := &Adapter{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
