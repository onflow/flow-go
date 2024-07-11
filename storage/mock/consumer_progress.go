// Code generated by mockery v2.21.4. DO NOT EDIT.

package mock

import mock "github.com/stretchr/testify/mock"

// ConsumerProgress is an autogenerated mock type for the ConsumerProgress type
type ConsumerProgress struct {
	mock.Mock
}

// Consumer provides a mock function with given fields:
func (_m *ConsumerProgress) Consumer() string {
	ret := _m.Called()

	var r0 string
	if rf, ok := ret.Get(0).(func() string); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(string)
	}

	return r0
}

// InitProcessedIndex provides a mock function with given fields: defaultIndex
func (_m *ConsumerProgress) InitProcessedIndex(defaultIndex uint64) error {
	ret := _m.Called(defaultIndex)

	var r0 error
	if rf, ok := ret.Get(0).(func(uint64) error); ok {
		r0 = rf(defaultIndex)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// ProcessedIndex provides a mock function with given fields:
func (_m *ConsumerProgress) ProcessedIndex() (uint64, error) {
	ret := _m.Called()

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

// SetProcessedIndex provides a mock function with given fields: processed
func (_m *ConsumerProgress) SetProcessedIndex(processed uint64) error {
	ret := _m.Called(processed)

	var r0 error
	if rf, ok := ret.Get(0).(func(uint64) error); ok {
		r0 = rf(processed)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

type mockConstructorTestingTNewConsumerProgress interface {
	mock.TestingT
	Cleanup(func())
}

// NewConsumerProgress creates a new instance of ConsumerProgress. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
func NewConsumerProgress(t mockConstructorTestingTNewConsumerProgress) *ConsumerProgress {
	mock := &ConsumerProgress{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
