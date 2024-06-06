// Code generated by mockery v2.43.2. DO NOT EDIT.

package mock

import (
	cluster "github.com/onflow/flow-go/model/cluster"
	flow "github.com/onflow/flow-go/model/flow"

	mock "github.com/stretchr/testify/mock"
)

// ClusterBlocks is an autogenerated mock type for the ClusterBlocks type
type ClusterBlocks struct {
	mock.Mock
}

// ByHeight provides a mock function with given fields: height
func (_m *ClusterBlocks) ByHeight(height uint64) (*cluster.Block, error) {
	ret := _m.Called(height)

	if len(ret) == 0 {
		panic("no return value specified for ByHeight")
	}

	var r0 *cluster.Block
	var r1 error
	if rf, ok := ret.Get(0).(func(uint64) (*cluster.Block, error)); ok {
		return rf(height)
	}
	if rf, ok := ret.Get(0).(func(uint64) *cluster.Block); ok {
		r0 = rf(height)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*cluster.Block)
		}
	}

	if rf, ok := ret.Get(1).(func(uint64) error); ok {
		r1 = rf(height)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// ByID provides a mock function with given fields: blockID
func (_m *ClusterBlocks) ByID(blockID flow.Identifier) (*cluster.Block, error) {
	ret := _m.Called(blockID)

	if len(ret) == 0 {
		panic("no return value specified for ByID")
	}

	var r0 *cluster.Block
	var r1 error
	if rf, ok := ret.Get(0).(func(flow.Identifier) (*cluster.Block, error)); ok {
		return rf(blockID)
	}
	if rf, ok := ret.Get(0).(func(flow.Identifier) *cluster.Block); ok {
		r0 = rf(blockID)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*cluster.Block)
		}
	}

	if rf, ok := ret.Get(1).(func(flow.Identifier) error); ok {
		r1 = rf(blockID)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// Store provides a mock function with given fields: block
func (_m *ClusterBlocks) Store(block *cluster.Block) error {
	ret := _m.Called(block)

	if len(ret) == 0 {
		panic("no return value specified for Store")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(*cluster.Block) error); ok {
		r0 = rf(block)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// NewClusterBlocks creates a new instance of ClusterBlocks. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewClusterBlocks(t interface {
	mock.TestingT
	Cleanup(func())
}) *ClusterBlocks {
	mock := &ClusterBlocks{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
