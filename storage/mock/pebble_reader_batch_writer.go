// Code generated by mockery v2.21.4. DO NOT EDIT.

package mock

import (
	pebble "github.com/cockroachdb/pebble"
	mock "github.com/stretchr/testify/mock"
)

// PebbleReaderBatchWriter is an autogenerated mock type for the PebbleReaderBatchWriter type
type PebbleReaderBatchWriter struct {
	mock.Mock
}

// ReaderWriter provides a mock function with given fields:
func (_m *PebbleReaderBatchWriter) ReaderWriter() (pebble.Reader, pebble.Writer) {
	ret := _m.Called()

	var r0 pebble.Reader
	var r1 pebble.Writer
	if rf, ok := ret.Get(0).(func() (pebble.Reader, pebble.Writer)); ok {
		return rf()
	}
	if rf, ok := ret.Get(0).(func() pebble.Reader); ok {
		r0 = rf()
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(pebble.Reader)
		}
	}

	if rf, ok := ret.Get(1).(func() pebble.Writer); ok {
		r1 = rf()
	} else {
		if ret.Get(1) != nil {
			r1 = ret.Get(1).(pebble.Writer)
		}
	}

	return r0, r1
}

type mockConstructorTestingTNewPebbleReaderBatchWriter interface {
	mock.TestingT
	Cleanup(func())
}

// NewPebbleReaderBatchWriter creates a new instance of PebbleReaderBatchWriter. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
func NewPebbleReaderBatchWriter(t mockConstructorTestingTNewPebbleReaderBatchWriter) *PebbleReaderBatchWriter {
	mock := &PebbleReaderBatchWriter{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
