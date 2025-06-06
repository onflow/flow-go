// Code generated by mockery v2.53.3. DO NOT EDIT.

package mocknetwork

import (
	host "github.com/libp2p/go-libp2p/core/host"
	mock "github.com/stretchr/testify/mock"

	p2p "github.com/onflow/flow-go/network/p2p"
)

// ConnectorFactory is an autogenerated mock type for the ConnectorFactory type
type ConnectorFactory struct {
	mock.Mock
}

// Execute provides a mock function with given fields: _a0
func (_m *ConnectorFactory) Execute(_a0 host.Host) (p2p.Connector, error) {
	ret := _m.Called(_a0)

	if len(ret) == 0 {
		panic("no return value specified for Execute")
	}

	var r0 p2p.Connector
	var r1 error
	if rf, ok := ret.Get(0).(func(host.Host) (p2p.Connector, error)); ok {
		return rf(_a0)
	}
	if rf, ok := ret.Get(0).(func(host.Host) p2p.Connector); ok {
		r0 = rf(_a0)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(p2p.Connector)
		}
	}

	if rf, ok := ret.Get(1).(func(host.Host) error); ok {
		r1 = rf(_a0)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// NewConnectorFactory creates a new instance of ConnectorFactory. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewConnectorFactory(t interface {
	mock.TestingT
	Cleanup(func())
}) *ConnectorFactory {
	mock := &ConnectorFactory{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
