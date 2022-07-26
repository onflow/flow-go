// Code generated by mockery v2.13.0. DO NOT EDIT.

package mocknetwork

import (
	flow "github.com/onflow/flow-go/model/flow"
	mock "github.com/stretchr/testify/mock"

	network "github.com/onflow/flow-go/network"
)

// Topology is an autogenerated mock type for the Topology type
type Topology struct {
	mock.Mock
}

// GenerateFanout provides a mock function with given fields: ids, channels
func (_m *Topology) Fanout(ids flow.IdentityList, channels network.ChannelList) (flow.IdentityList, error) {
	ret := _m.Called(ids, channels)

	var r0 flow.IdentityList
	if rf, ok := ret.Get(0).(func(flow.IdentityList, network.ChannelList) flow.IdentityList); ok {
		r0 = rf(ids, channels)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(flow.IdentityList)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(flow.IdentityList, network.ChannelList) error); ok {
		r1 = rf(ids, channels)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

type NewTopologyT interface {
	mock.TestingT
	Cleanup(func())
}

// NewTopology creates a new instance of Topology. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
func NewTopology(t NewTopologyT) *Topology {
	mock := &Topology{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
