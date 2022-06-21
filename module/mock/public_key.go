// Code generated by mockery v2.13.0. DO NOT EDIT.

package mock

import (
	crypto "github.com/onflow/flow-go/crypto"
	hash "github.com/onflow/flow-go/crypto/hash"

	mock "github.com/stretchr/testify/mock"
)

// PublicKey is an autogenerated mock type for the PublicKey type
type PublicKey struct {
	mock.Mock
}

// Algorithm provides a mock function with given fields:
func (_m *PublicKey) Algorithm() crypto.SigningAlgorithm {
	ret := _m.Called()

	var r0 crypto.SigningAlgorithm
	if rf, ok := ret.Get(0).(func() crypto.SigningAlgorithm); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(crypto.SigningAlgorithm)
	}

	return r0
}

// Encode provides a mock function with given fields:
func (_m *PublicKey) Encode() []byte {
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

// EncodeCompressed provides a mock function with given fields:
func (_m *PublicKey) EncodeCompressed() []byte {
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

// Equals provides a mock function with given fields: _a0
func (_m *PublicKey) Equals(_a0 crypto.PublicKey) bool {
	ret := _m.Called(_a0)

	var r0 bool
	if rf, ok := ret.Get(0).(func(crypto.PublicKey) bool); ok {
		r0 = rf(_a0)
	} else {
		r0 = ret.Get(0).(bool)
	}

	return r0
}

// Size provides a mock function with given fields:
func (_m *PublicKey) Size() int {
	ret := _m.Called()

	var r0 int
	if rf, ok := ret.Get(0).(func() int); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(int)
	}

	return r0
}

// String provides a mock function with given fields:
func (_m *PublicKey) String() string {
	ret := _m.Called()

	var r0 string
	if rf, ok := ret.Get(0).(func() string); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(string)
	}

	return r0
}

// Verify provides a mock function with given fields: _a0, _a1, _a2
func (_m *PublicKey) Verify(_a0 crypto.Signature, _a1 []byte, _a2 hash.Hasher) (bool, error) {
	ret := _m.Called(_a0, _a1, _a2)

	var r0 bool
	if rf, ok := ret.Get(0).(func(crypto.Signature, []byte, hash.Hasher) bool); ok {
		r0 = rf(_a0, _a1, _a2)
	} else {
		r0 = ret.Get(0).(bool)
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(crypto.Signature, []byte, hash.Hasher) error); ok {
		r1 = rf(_a0, _a1, _a2)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

type NewPublicKeyT interface {
	mock.TestingT
	Cleanup(func())
}

// NewPublicKey creates a new instance of PublicKey. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
func NewPublicKey(t NewPublicKeyT) *PublicKey {
	mock := &PublicKey{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
