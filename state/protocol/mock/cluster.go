// Code generated by mockery v2.13.0. DO NOT EDIT.

package mock

import (
	cluster "github.com/onflow/flow-go/model/cluster"
	flow "github.com/onflow/flow-go/model/flow"

	mock "github.com/stretchr/testify/mock"
)

// Cluster is an autogenerated mock type for the Cluster type
type Cluster struct {
	mock.Mock
}

// ChainID provides a mock function with given fields:
func (_m *Cluster) ChainID() flow.ChainID {
	ret := _m.Called()

	var r0 flow.ChainID
	if rf, ok := ret.Get(0).(func() flow.ChainID); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(flow.ChainID)
	}

	return r0
}

// EpochCounter provides a mock function with given fields:
func (_m *Cluster) EpochCounter() uint64 {
	ret := _m.Called()

	var r0 uint64
	if rf, ok := ret.Get(0).(func() uint64); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(uint64)
	}

	return r0
}

// Index provides a mock function with given fields:
func (_m *Cluster) Index() uint {
	ret := _m.Called()

	var r0 uint
	if rf, ok := ret.Get(0).(func() uint); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(uint)
	}

	return r0
}

// Members provides a mock function with given fields:
func (_m *Cluster) Members() flow.IdentityList {
	ret := _m.Called()

	var r0 flow.IdentityList
	if rf, ok := ret.Get(0).(func() flow.IdentityList); ok {
		r0 = rf()
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(flow.IdentityList)
		}
	}

	return r0
}

// RootBlock provides a mock function with given fields:
func (_m *Cluster) RootBlock() *cluster.Block {
	ret := _m.Called()

	var r0 *cluster.Block
	if rf, ok := ret.Get(0).(func() *cluster.Block); ok {
		r0 = rf()
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*cluster.Block)
		}
	}

	return r0
}

// RootQC provides a mock function with given fields:
func (_m *Cluster) RootQC() *flow.QuorumCertificate {
	ret := _m.Called()

	var r0 *flow.QuorumCertificate
	if rf, ok := ret.Get(0).(func() *flow.QuorumCertificate); ok {
		r0 = rf()
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*flow.QuorumCertificate)
		}
	}

	return r0
}

type NewClusterT interface {
	mock.TestingT
	Cleanup(func())
}

// NewCluster creates a new instance of Cluster. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
func NewCluster(t NewClusterT) *Cluster {
	mock := &Cluster{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
