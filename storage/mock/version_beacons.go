// Code generated by mockery v2.13.1. DO NOT EDIT.

package mock

import (
	flow "github.com/onflow/flow-go/model/flow"
	mock "github.com/stretchr/testify/mock"
)

// VersionBeacons is an autogenerated mock type for the VersionBeacons type
type VersionBeacons struct {
	mock.Mock
}

// Highest provides a mock function with given fields: maxHeight
func (_m *VersionBeacons) Highest(maxHeight uint64) (*flow.VersionBeacon, uint64, error) {
	ret := _m.Called(maxHeight)

	var r0 *flow.VersionBeacon
	if rf, ok := ret.Get(0).(func(uint64) *flow.VersionBeacon); ok {
		r0 = rf(maxHeight)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*flow.VersionBeacon)
		}
	}

	var r1 uint64
	if rf, ok := ret.Get(1).(func(uint64) uint64); ok {
		r1 = rf(maxHeight)
	} else {
		r1 = ret.Get(1).(uint64)
	}

	var r2 error
	if rf, ok := ret.Get(2).(func(uint64) error); ok {
		r2 = rf(maxHeight)
	} else {
		r2 = ret.Error(2)
	}

	return r0, r1, r2
}

type mockConstructorTestingTNewVersionBeacons interface {
	mock.TestingT
	Cleanup(func())
}

// NewVersionBeacons creates a new instance of VersionBeacons. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
func NewVersionBeacons(t mockConstructorTestingTNewVersionBeacons) *VersionBeacons {
	mock := &VersionBeacons{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
