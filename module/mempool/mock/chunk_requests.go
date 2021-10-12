// Code generated by mockery v1.0.0. DO NOT EDIT.

package mempool

import (
	flow "github.com/onflow/flow-go/model/flow"
	mempool "github.com/onflow/flow-go/module/mempool"

	mock "github.com/stretchr/testify/mock"

	time "time"

	verification "github.com/onflow/flow-go/model/verification"
)

// ChunkRequests is an autogenerated mock type for the ChunkRequests type
type ChunkRequests struct {
	mock.Mock
}

// Add provides a mock function with given fields: request
func (_m *ChunkRequests) Add(request *verification.ChunkDataPackRequest) bool {
	ret := _m.Called(request)

	var r0 bool
	if rf, ok := ret.Get(0).(func(*verification.ChunkDataPackRequest) bool); ok {
		r0 = rf(request)
	} else {
		r0 = ret.Get(0).(bool)
	}

	return r0
}

// All provides a mock function with given fields:
func (_m *ChunkRequests) All() []*verification.ChunkDataPackRequest {
	ret := _m.Called()

	var r0 []*verification.ChunkDataPackRequest
	if rf, ok := ret.Get(0).(func() []*verification.ChunkDataPackRequest); ok {
		r0 = rf()
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]*verification.ChunkDataPackRequest)
		}
	}

	return r0
}

// ByID provides a mock function with given fields: chunkID
func (_m *ChunkRequests) ByID(chunkID flow.Identifier) (*verification.ChunkDataPackRequest, bool) {
	ret := _m.Called(chunkID)

	var r0 *verification.ChunkDataPackRequest
	if rf, ok := ret.Get(0).(func(flow.Identifier) *verification.ChunkDataPackRequest); ok {
		r0 = rf(chunkID)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*verification.ChunkDataPackRequest)
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

// GetAndRemove provides a mock function with given fields: chunkID
func (_m *ChunkRequests) GetAndRemove(chunkID flow.Identifier) (*verification.ChunkDataPackRequest, bool) {
	ret := _m.Called(chunkID)

	var r0 *verification.ChunkDataPackRequest
	if rf, ok := ret.Get(0).(func(flow.Identifier) *verification.ChunkDataPackRequest); ok {
		r0 = rf(chunkID)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*verification.ChunkDataPackRequest)
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

// IncrementAttempt provides a mock function with given fields: chunkID
func (_m *ChunkRequests) IncrementAttempt(chunkID flow.Identifier) bool {
	ret := _m.Called(chunkID)

	var r0 bool
	if rf, ok := ret.Get(0).(func(flow.Identifier) bool); ok {
		r0 = rf(chunkID)
	} else {
		r0 = ret.Get(0).(bool)
	}

	return r0
}

// Rem provides a mock function with given fields: chunkID
func (_m *ChunkRequests) Rem(chunkID flow.Identifier) bool {
	ret := _m.Called(chunkID)

	var r0 bool
	if rf, ok := ret.Get(0).(func(flow.Identifier) bool); ok {
		r0 = rf(chunkID)
	} else {
		r0 = ret.Get(0).(bool)
	}

	return r0
}

// RequestHistory provides a mock function with given fields: chunkID
func (_m *ChunkRequests) RequestHistory(chunkID flow.Identifier) (uint64, time.Time, time.Duration, bool) {
	ret := _m.Called(chunkID)

	var r0 uint64
	if rf, ok := ret.Get(0).(func(flow.Identifier) uint64); ok {
		r0 = rf(chunkID)
	} else {
		r0 = ret.Get(0).(uint64)
	}

	var r1 time.Time
	if rf, ok := ret.Get(1).(func(flow.Identifier) time.Time); ok {
		r1 = rf(chunkID)
	} else {
		r1 = ret.Get(1).(time.Time)
	}

	var r2 time.Duration
	if rf, ok := ret.Get(2).(func(flow.Identifier) time.Duration); ok {
		r2 = rf(chunkID)
	} else {
		r2 = ret.Get(2).(time.Duration)
	}

	var r3 bool
	if rf, ok := ret.Get(3).(func(flow.Identifier) bool); ok {
		r3 = rf(chunkID)
	} else {
		r3 = ret.Get(3).(bool)
	}

	return r0, r1, r2, r3
}

// Size provides a mock function with given fields:
func (_m *ChunkRequests) Size() uint {
	ret := _m.Called()

	var r0 uint
	if rf, ok := ret.Get(0).(func() uint); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(uint)
	}

	return r0
}

// UpdateRequestHistory provides a mock function with given fields: chunkID, updater
func (_m *ChunkRequests) UpdateRequestHistory(chunkID flow.Identifier, updater mempool.ChunkRequestHistoryUpdaterFunc) (uint64, time.Time, time.Duration, bool) {
	ret := _m.Called(chunkID, updater)

	var r0 uint64
	if rf, ok := ret.Get(0).(func(flow.Identifier, mempool.ChunkRequestHistoryUpdaterFunc) uint64); ok {
		r0 = rf(chunkID, updater)
	} else {
		r0 = ret.Get(0).(uint64)
	}

	var r1 time.Time
	if rf, ok := ret.Get(1).(func(flow.Identifier, mempool.ChunkRequestHistoryUpdaterFunc) time.Time); ok {
		r1 = rf(chunkID, updater)
	} else {
		r1 = ret.Get(1).(time.Time)
	}

	var r2 time.Duration
	if rf, ok := ret.Get(2).(func(flow.Identifier, mempool.ChunkRequestHistoryUpdaterFunc) time.Duration); ok {
		r2 = rf(chunkID, updater)
	} else {
		r2 = ret.Get(2).(time.Duration)
	}

	var r3 bool
	if rf, ok := ret.Get(3).(func(flow.Identifier, mempool.ChunkRequestHistoryUpdaterFunc) bool); ok {
		r3 = rf(chunkID, updater)
	} else {
		r3 = ret.Get(3).(bool)
	}

	return r0, r1, r2, r3
}
