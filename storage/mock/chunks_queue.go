// Code generated by mockery v2.53.3. DO NOT EDIT.

package mock

import (
	chunks "github.com/onflow/flow-go/model/chunks"
	mock "github.com/stretchr/testify/mock"
)

// ChunksQueue is an autogenerated mock type for the ChunksQueue type
type ChunksQueue struct {
	mock.Mock
}

// AtIndex provides a mock function with given fields: index
func (_m *ChunksQueue) AtIndex(index uint64) (*chunks.Locator, error) {
	ret := _m.Called(index)

	if len(ret) == 0 {
		panic("no return value specified for AtIndex")
	}

	var r0 *chunks.Locator
	var r1 error
	if rf, ok := ret.Get(0).(func(uint64) (*chunks.Locator, error)); ok {
		return rf(index)
	}
	if rf, ok := ret.Get(0).(func(uint64) *chunks.Locator); ok {
		r0 = rf(index)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*chunks.Locator)
		}
	}

	if rf, ok := ret.Get(1).(func(uint64) error); ok {
		r1 = rf(index)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// LatestIndex provides a mock function with no fields
func (_m *ChunksQueue) LatestIndex() (uint64, error) {
	ret := _m.Called()

	if len(ret) == 0 {
		panic("no return value specified for LatestIndex")
	}

	var r0 uint64
	var r1 error
	if rf, ok := ret.Get(0).(func() (uint64, error)); ok {
		return rf()
	}
	if rf, ok := ret.Get(0).(func() uint64); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(uint64)
	}

	if rf, ok := ret.Get(1).(func() error); ok {
		r1 = rf()
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// StoreChunkLocator provides a mock function with given fields: locator
func (_m *ChunksQueue) StoreChunkLocator(locator *chunks.Locator) (bool, error) {
	ret := _m.Called(locator)

	if len(ret) == 0 {
		panic("no return value specified for StoreChunkLocator")
	}

	var r0 bool
	var r1 error
	if rf, ok := ret.Get(0).(func(*chunks.Locator) (bool, error)); ok {
		return rf(locator)
	}
	if rf, ok := ret.Get(0).(func(*chunks.Locator) bool); ok {
		r0 = rf(locator)
	} else {
		r0 = ret.Get(0).(bool)
	}

	if rf, ok := ret.Get(1).(func(*chunks.Locator) error); ok {
		r1 = rf(locator)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// NewChunksQueue creates a new instance of ChunksQueue. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewChunksQueue(t interface {
	mock.TestingT
	Cleanup(func())
}) *ChunksQueue {
	mock := &ChunksQueue{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
