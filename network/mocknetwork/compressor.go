// Code generated by mockery v2.13.1. DO NOT EDIT.

package mocknetwork

import (
	io "io"

	network "github.com/onflow/flow-go/network"
	mock "github.com/stretchr/testify/mock"
)

// Compressor is an autogenerated mock type for the Compressor type
type Compressor struct {
	mock.Mock
}

// NewReader provides a mock function with given fields: _a0
func (_m *Compressor) NewReader(_a0 io.Reader) (io.ReadCloser, error) {
	ret := _m.Called(_a0)

	var r0 io.ReadCloser
	if rf, ok := ret.Get(0).(func(io.Reader) io.ReadCloser); ok {
		r0 = rf(_a0)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(io.ReadCloser)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(io.Reader) error); ok {
		r1 = rf(_a0)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// NewWriter provides a mock function with given fields: _a0
func (_m *Compressor) NewWriter(_a0 io.Writer) (network.WriteCloseFlusher, error) {
	ret := _m.Called(_a0)

	var r0 network.WriteCloseFlusher
	if rf, ok := ret.Get(0).(func(io.Writer) network.WriteCloseFlusher); ok {
		r0 = rf(_a0)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(network.WriteCloseFlusher)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(io.Writer) error); ok {
		r1 = rf(_a0)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

type mockConstructorTestingTNewCompressor interface {
	mock.TestingT
	Cleanup(func())
}

// NewCompressor creates a new instance of Compressor. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
func NewCompressor(t mockConstructorTestingTNewCompressor) *Compressor {
	mock := &Compressor{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
