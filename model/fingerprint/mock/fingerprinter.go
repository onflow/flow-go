// Code generated by mockery v2.12.3. DO NOT EDIT.

package mock

import mock "github.com/stretchr/testify/mock"

// Fingerprinter is an autogenerated mock type for the Fingerprinter type
type Fingerprinter struct {
	mock.Mock
}

// Fingerprint provides a mock function with given fields:
func (_m *Fingerprinter) Fingerprint() []byte {
	ret := _m.Called()

	var r0 []byte
	if rf, ok := ret.Get(0).(func() []byte); ok {
		r0 = rf()
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]byte)
		}
	}

	return r0
}

type NewFingerprinterT interface {
	mock.TestingT
	Cleanup(func())
}

// NewFingerprinter creates a new instance of Fingerprinter. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
func NewFingerprinter(t NewFingerprinterT) *Fingerprinter {
	mock := &Fingerprinter{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
