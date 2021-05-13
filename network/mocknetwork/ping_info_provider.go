// Code generated by mockery v1.0.0. DO NOT EDIT.

package mocknetwork

import mock "github.com/stretchr/testify/mock"

// PingInfoProvider is an autogenerated mock type for the PingInfoProvider type
type PingInfoProvider struct {
	mock.Mock
}

// SealedBlockHeight provides a mock function with given fields:
func (_m *PingInfoProvider) SealedBlockHeight() uint64 {
	ret := _m.Called()

	var r0 uint64
	if rf, ok := ret.Get(0).(func() uint64); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(uint64)
	}

	return r0
}

// SoftwareVersion provides a mock function with given fields:
func (_m *PingInfoProvider) SoftwareVersion() string {
	ret := _m.Called()

	var r0 string
	if rf, ok := ret.Get(0).(func() string); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(string)
	}

	return r0
}
