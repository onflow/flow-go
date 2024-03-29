// Code generated by mockery v2.21.4. DO NOT EDIT.

package mockp2p

import (
	channels "github.com/onflow/flow-go/network/channels"
	mock "github.com/stretchr/testify/mock"

	p2p "github.com/onflow/flow-go/network/p2p"
)

// Subscriptions is an autogenerated mock type for the Subscriptions type
type Subscriptions struct {
	mock.Mock
}

// HasSubscription provides a mock function with given fields: topic
func (_m *Subscriptions) HasSubscription(topic channels.Topic) bool {
	ret := _m.Called(topic)

	var r0 bool
	if rf, ok := ret.Get(0).(func(channels.Topic) bool); ok {
		r0 = rf(topic)
	} else {
		r0 = ret.Get(0).(bool)
	}

	return r0
}

// SetUnicastManager provides a mock function with given fields: uniMgr
func (_m *Subscriptions) SetUnicastManager(uniMgr p2p.UnicastManager) {
	_m.Called(uniMgr)
}

type mockConstructorTestingTNewSubscriptions interface {
	mock.TestingT
	Cleanup(func())
}

// NewSubscriptions creates a new instance of Subscriptions. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
func NewSubscriptions(t mockConstructorTestingTNewSubscriptions) *Subscriptions {
	mock := &Subscriptions{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
