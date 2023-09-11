// Code generated by mockery v2.21.4. DO NOT EDIT.

package mock

import (
	context "context"

	flow "github.com/onflow/flow-go/model/flow"
	mock "github.com/stretchr/testify/mock"

	snapshot "github.com/onflow/flow-go/fvm/storage/snapshot"
)

// ReadOnlyExecutionState is an autogenerated mock type for the ReadOnlyExecutionState type
type ReadOnlyExecutionState struct {
	mock.Mock
}

// ChunkDataPackByChunkID provides a mock function with given fields: _a0
func (_m *ReadOnlyExecutionState) ChunkDataPackByChunkID(_a0 flow.Identifier) (*flow.ChunkDataPack, error) {
	ret := _m.Called(_a0)

	var r0 *flow.ChunkDataPack
	var r1 error
	if rf, ok := ret.Get(0).(func(flow.Identifier) (*flow.ChunkDataPack, error)); ok {
		return rf(_a0)
	}
	if rf, ok := ret.Get(0).(func(flow.Identifier) *flow.ChunkDataPack); ok {
		r0 = rf(_a0)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*flow.ChunkDataPack)
		}
	}

	if rf, ok := ret.Get(1).(func(flow.Identifier) error); ok {
		r1 = rf(_a0)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetExecutionResultID provides a mock function with given fields: _a0, _a1
func (_m *ReadOnlyExecutionState) GetExecutionResultID(_a0 context.Context, _a1 flow.Identifier) (flow.Identifier, error) {
	ret := _m.Called(_a0, _a1)

	var r0 flow.Identifier
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, flow.Identifier) (flow.Identifier, error)); ok {
		return rf(_a0, _a1)
	}
	if rf, ok := ret.Get(0).(func(context.Context, flow.Identifier) flow.Identifier); ok {
		r0 = rf(_a0, _a1)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(flow.Identifier)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, flow.Identifier) error); ok {
		r1 = rf(_a0, _a1)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetHighestFinalizedExecuted provides a mock function with given fields:
func (_m *ReadOnlyExecutionState) GetHighestFinalizedExecuted() uint64 {
	ret := _m.Called()

	var r0 uint64
	if rf, ok := ret.Get(0).(func() uint64); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(uint64)
	}

	return r0
}

// HasState provides a mock function with given fields: _a0
func (_m *ReadOnlyExecutionState) HasState(_a0 flow.StateCommitment) bool {
	ret := _m.Called(_a0)

	var r0 bool
	if rf, ok := ret.Get(0).(func(flow.StateCommitment) bool); ok {
		r0 = rf(_a0)
	} else {
		r0 = ret.Get(0).(bool)
	}

	return r0
}

// IsBlockExecuted provides a mock function with given fields: height, blockID
func (_m *ReadOnlyExecutionState) IsBlockExecuted(height uint64, blockID flow.Identifier) (bool, error) {
	ret := _m.Called(height, blockID)

	var r0 bool
	var r1 error
	if rf, ok := ret.Get(0).(func(uint64, flow.Identifier) (bool, error)); ok {
		return rf(height, blockID)
	}
	if rf, ok := ret.Get(0).(func(uint64, flow.Identifier) bool); ok {
		r0 = rf(height, blockID)
	} else {
		r0 = ret.Get(0).(bool)
	}

	if rf, ok := ret.Get(1).(func(uint64, flow.Identifier) error); ok {
		r1 = rf(height, blockID)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// NewStorageSnapshot provides a mock function with given fields: blockID, height
func (_m *ReadOnlyExecutionState) NewStorageSnapshot(blockID flow.Identifier, height uint64) snapshot.StorageSnapshot {
	ret := _m.Called(blockID, height)

	var r0 snapshot.StorageSnapshot
	if rf, ok := ret.Get(0).(func(flow.Identifier, uint64) snapshot.StorageSnapshot); ok {
		r0 = rf(blockID, height)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(snapshot.StorageSnapshot)
		}
	}

	return r0
}

// StateCommitmentByBlockID provides a mock function with given fields: _a0, _a1
func (_m *ReadOnlyExecutionState) StateCommitmentByBlockID(_a0 context.Context, _a1 flow.Identifier) (flow.StateCommitment, error) {
	ret := _m.Called(_a0, _a1)

	var r0 flow.StateCommitment
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, flow.Identifier) (flow.StateCommitment, error)); ok {
		return rf(_a0, _a1)
	}
	if rf, ok := ret.Get(0).(func(context.Context, flow.Identifier) flow.StateCommitment); ok {
		r0 = rf(_a0, _a1)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(flow.StateCommitment)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, flow.Identifier) error); ok {
		r1 = rf(_a0, _a1)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

type mockConstructorTestingTNewReadOnlyExecutionState interface {
	mock.TestingT
	Cleanup(func())
}

// NewReadOnlyExecutionState creates a new instance of ReadOnlyExecutionState. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
func NewReadOnlyExecutionState(t mockConstructorTestingTNewReadOnlyExecutionState) *ReadOnlyExecutionState {
	mock := &ReadOnlyExecutionState{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
