// Code generated by mockery v2.12.1. DO NOT EDIT.

package mockinsecure

import (
	context "context"

	insecure "github.com/onflow/flow-go/insecure"
	metadata "google.golang.org/grpc/metadata"

	mock "github.com/stretchr/testify/mock"

	testing "testing"
)

// CorruptibleConduitFactory_RegisterAttackerClient is an autogenerated mock type for the CorruptibleConduitFactory_RegisterAttackerClient type
type CorruptibleConduitFactory_RegisterAttackerClient struct {
	mock.Mock
}

// CloseSend provides a mock function with given fields:
func (_m *CorruptibleConduitFactory_RegisterAttackerClient) CloseSend() error {
	ret := _m.Called()

	var r0 error
	if rf, ok := ret.Get(0).(func() error); ok {
		r0 = rf()
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// Context provides a mock function with given fields:
func (_m *CorruptibleConduitFactory_RegisterAttackerClient) Context() context.Context {
	ret := _m.Called()

	var r0 context.Context
	if rf, ok := ret.Get(0).(func() context.Context); ok {
		r0 = rf()
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(context.Context)
		}
	}

	return r0
}

// Header provides a mock function with given fields:
func (_m *CorruptibleConduitFactory_RegisterAttackerClient) Header() (metadata.MD, error) {
	ret := _m.Called()

	var r0 metadata.MD
	if rf, ok := ret.Get(0).(func() metadata.MD); ok {
		r0 = rf()
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(metadata.MD)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func() error); ok {
		r1 = rf()
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// Recv provides a mock function with given fields:
func (_m *CorruptibleConduitFactory_RegisterAttackerClient) Recv() (*insecure.Message, error) {
	ret := _m.Called()

	var r0 *insecure.Message
	if rf, ok := ret.Get(0).(func() *insecure.Message); ok {
		r0 = rf()
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*insecure.Message)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func() error); ok {
		r1 = rf()
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// RecvMsg provides a mock function with given fields: m
func (_m *CorruptibleConduitFactory_RegisterAttackerClient) RecvMsg(m interface{}) error {
	ret := _m.Called(m)

	var r0 error
	if rf, ok := ret.Get(0).(func(interface{}) error); ok {
		r0 = rf(m)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// SendMsg provides a mock function with given fields: m
func (_m *CorruptibleConduitFactory_RegisterAttackerClient) SendMsg(m interface{}) error {
	ret := _m.Called(m)

	var r0 error
	if rf, ok := ret.Get(0).(func(interface{}) error); ok {
		r0 = rf(m)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// Trailer provides a mock function with given fields:
func (_m *CorruptibleConduitFactory_RegisterAttackerClient) Trailer() metadata.MD {
	ret := _m.Called()

	var r0 metadata.MD
	if rf, ok := ret.Get(0).(func() metadata.MD); ok {
		r0 = rf()
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(metadata.MD)
		}
	}

	return r0
}

// NewCorruptibleConduitFactory_RegisterAttackerClient creates a new instance of CorruptibleConduitFactory_RegisterAttackerClient. It also registers the testing.TB interface on the mock and a cleanup function to assert the mocks expectations.
func NewCorruptibleConduitFactory_RegisterAttackerClient(t testing.TB) *CorruptibleConduitFactory_RegisterAttackerClient {
	mock := &CorruptibleConduitFactory_RegisterAttackerClient{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
