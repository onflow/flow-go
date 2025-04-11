// Code generated by mockery v2.43.2. DO NOT EDIT.

package mock

import (
	storage "github.com/onflow/flow-go/storage"
	mock "github.com/stretchr/testify/mock"
)

// DB is an autogenerated mock type for the DB type
type DB struct {
	mock.Mock
}

// NewBatch provides a mock function with given fields:
func (_m *DB) NewBatch() storage.Batch {
	ret := _m.Called()

	if len(ret) == 0 {
		panic("no return value specified for NewBatch")
	}

	var r0 storage.Batch
	if rf, ok := ret.Get(0).(func() storage.Batch); ok {
		r0 = rf()
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(storage.Batch)
		}
	}

	return r0
}

// Reader provides a mock function with given fields:
func (_m *DB) Reader() storage.Reader {
	ret := _m.Called()

	if len(ret) == 0 {
		panic("no return value specified for Reader")
	}

	var r0 storage.Reader
	if rf, ok := ret.Get(0).(func() storage.Reader); ok {
		r0 = rf()
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(storage.Reader)
		}
	}

	return r0
}

// WithReaderBatchWriter provides a mock function with given fields: _a0
func (_m *DB) WithReaderBatchWriter(_a0 func(storage.ReaderBatchWriter) error) error {
	ret := _m.Called(_a0)

	if len(ret) == 0 {
		panic("no return value specified for WithReaderBatchWriter")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(func(storage.ReaderBatchWriter) error) error); ok {
		r0 = rf(_a0)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// NewDB creates a new instance of DB. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewDB(t interface {
	mock.TestingT
	Cleanup(func())
}) *DB {
	mock := &DB{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
