// Code generated by mockery v2.21.4. DO NOT EDIT.

package mock

import (
	flow "github.com/onflow/flow-go/model/flow"
	mock "github.com/stretchr/testify/mock"
)

// ComputationResultUploadStatus is an autogenerated mock type for the ComputationResultUploadStatus type
type ComputationResultUploadStatus struct {
	mock.Mock
}

// ByID provides a mock function with given fields: blockID
func (_m *ComputationResultUploadStatus) ByID(blockID flow.Identifier) (bool, error) {
	ret := _m.Called(blockID)

	var r0 bool
	var r1 error
	if rf, ok := ret.Get(0).(func(flow.Identifier) (bool, error)); ok {
		return rf(blockID)
	}
	if rf, ok := ret.Get(0).(func(flow.Identifier) bool); ok {
		r0 = rf(blockID)
	} else {
		r0 = ret.Get(0).(bool)
	}

	if rf, ok := ret.Get(1).(func(flow.Identifier) error); ok {
		r1 = rf(blockID)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetIDsByUploadStatus provides a mock function with given fields: targetUploadStatus
func (_m *ComputationResultUploadStatus) GetIDsByUploadStatus(targetUploadStatus bool) ([]flow.Identifier, error) {
	ret := _m.Called(targetUploadStatus)

	var r0 []flow.Identifier
	var r1 error
	if rf, ok := ret.Get(0).(func(bool) ([]flow.Identifier, error)); ok {
		return rf(targetUploadStatus)
	}
	if rf, ok := ret.Get(0).(func(bool) []flow.Identifier); ok {
		r0 = rf(targetUploadStatus)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]flow.Identifier)
		}
	}

	if rf, ok := ret.Get(1).(func(bool) error); ok {
		r1 = rf(targetUploadStatus)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// Remove provides a mock function with given fields: blockID
func (_m *ComputationResultUploadStatus) Remove(blockID flow.Identifier) error {
	ret := _m.Called(blockID)

	var r0 error
	if rf, ok := ret.Get(0).(func(flow.Identifier) error); ok {
		r0 = rf(blockID)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// Upsert provides a mock function with given fields: blockID, wasUploadCompleted
func (_m *ComputationResultUploadStatus) Upsert(blockID flow.Identifier, wasUploadCompleted bool) error {
	ret := _m.Called(blockID, wasUploadCompleted)

	var r0 error
	if rf, ok := ret.Get(0).(func(flow.Identifier, bool) error); ok {
		r0 = rf(blockID, wasUploadCompleted)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

type mockConstructorTestingTNewComputationResultUploadStatus interface {
	mock.TestingT
	Cleanup(func())
}

// NewComputationResultUploadStatus creates a new instance of ComputationResultUploadStatus. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
func NewComputationResultUploadStatus(t mockConstructorTestingTNewComputationResultUploadStatus) *ComputationResultUploadStatus {
	mock := &ComputationResultUploadStatus{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
