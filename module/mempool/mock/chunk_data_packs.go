// Code generated by mockery v2.13.1. DO NOT EDIT.

package mempool

import (
	flow "github.com/onflow/flow-go/model/flow"

	mock "github.com/stretchr/testify/mock"
)

// ChunkDataPacks is an autogenerated mock type for the ChunkDataPacks type
type ChunkDataPacks struct {
	mock.Mock
}

// Add provides a mock function with given fields: cdp
func (_m *ChunkDataPacks) Add(cdp *flow.ChunkDataPack) bool {
	ret := _m.Called(cdp)

	var r0 bool
	if rf, ok := ret.Get(0).(func(*flow.ChunkDataPack) bool); ok {
		r0 = rf(cdp)
	} else {
		r0 = ret.Get(0).(bool)
	}

	return r0
}

// All provides a mock function with given fields:
func (_m *ChunkDataPacks) All() []*flow.ChunkDataPack {
	ret := _m.Called()

	var r0 []*flow.ChunkDataPack
	if rf, ok := ret.Get(0).(func() []*flow.ChunkDataPack); ok {
		r0 = rf()
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]*flow.ChunkDataPack)
		}
	}

	return r0
}

// ByChunkID provides a mock function with given fields: chunkID
func (_m *ChunkDataPacks) ByChunkID(chunkID flow.Identifier) (*flow.ChunkDataPack, bool) {
	ret := _m.Called(chunkID)

	var r0 *flow.ChunkDataPack
	if rf, ok := ret.Get(0).(func(flow.Identifier) *flow.ChunkDataPack); ok {
		r0 = rf(chunkID)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*flow.ChunkDataPack)
		}
	}

	var r1 bool
	if rf, ok := ret.Get(1).(func(flow.Identifier) bool); ok {
		r1 = rf(chunkID)
	} else {
		r1 = ret.Get(1).(bool)
	}

	return r0, r1
}

// Has provides a mock function with given fields: chunkID
func (_m *ChunkDataPacks) Has(chunkID flow.Identifier) bool {
	ret := _m.Called(chunkID)

	var r0 bool
	if rf, ok := ret.Get(0).(func(flow.Identifier) bool); ok {
		r0 = rf(chunkID)
	} else {
		r0 = ret.Get(0).(bool)
	}

	return r0
}

// Remove provides a mock function with given fields: chunkID
func (_m *ChunkDataPacks) Remove(chunkID flow.Identifier) bool {
	ret := _m.Called(chunkID)

	var r0 bool
	if rf, ok := ret.Get(0).(func(flow.Identifier) bool); ok {
		r0 = rf(chunkID)
	} else {
		r0 = ret.Get(0).(bool)
	}

	return r0
}

// Size provides a mock function with given fields:
func (_m *ChunkDataPacks) Size() uint {
	ret := _m.Called()

	var r0 uint
	if rf, ok := ret.Get(0).(func() uint); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(uint)
	}

	return r0
}

type mockConstructorTestingTNewChunkDataPacks interface {
	mock.TestingT
	Cleanup(func())
}

// NewChunkDataPacks creates a new instance of ChunkDataPacks. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
func NewChunkDataPacks(t mockConstructorTestingTNewChunkDataPacks) *ChunkDataPacks {
	mock := &ChunkDataPacks{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
