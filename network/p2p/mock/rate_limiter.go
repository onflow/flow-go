// Code generated by mockery v2.21.4. DO NOT EDIT.

package mockp2p

import (
	p2p "github.com/onflow/flow-go/network/p2p"
	mock "github.com/stretchr/testify/mock"

	peer "github.com/libp2p/go-libp2p/core/peer"
)

// RateLimiter is an autogenerated mock type for the RateLimiter type
type RateLimiter struct {
	mock.Mock
}

// Allow provides a mock function with given fields: peerID, msgSize
func (_m *RateLimiter) Allow(peerID peer.ID, msgSize int) bool {
	ret := _m.Called(peerID, msgSize)

	var r0 bool
	if rf, ok := ret.Get(0).(func(peer.ID, int) bool); ok {
		r0 = rf(peerID, msgSize)
	} else {
		r0 = ret.Get(0).(bool)
	}

	return r0
}

// IsRateLimited provides a mock function with given fields: peerID
func (_m *RateLimiter) IsRateLimited(peerID peer.ID) bool {
	ret := _m.Called(peerID)

	var r0 bool
	if rf, ok := ret.Get(0).(func(peer.ID) bool); ok {
		r0 = rf(peerID)
	} else {
		r0 = ret.Get(0).(bool)
	}

	return r0
}

// SetTimeNowFunc provides a mock function with given fields: now
func (_m *RateLimiter) SetTimeNowFunc(now p2p.GetTimeNow) {
	_m.Called(now)
}

// Start provides a mock function with given fields:
func (_m *RateLimiter) Start() {
	_m.Called()
}

// Stop provides a mock function with given fields:
func (_m *RateLimiter) Stop() {
	_m.Called()
}

type mockConstructorTestingTNewRateLimiter interface {
	mock.TestingT
	Cleanup(func())
}

// NewRateLimiter creates a new instance of RateLimiter. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
func NewRateLimiter(t mockConstructorTestingTNewRateLimiter) *RateLimiter {
	mock := &RateLimiter{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
