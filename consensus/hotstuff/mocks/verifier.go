// Code generated by mockery v2.21.4. DO NOT EDIT.

package mocks

import (
	flow "github.com/onflow/flow-go/model/flow"

	mock "github.com/stretchr/testify/mock"
)

// Verifier is an autogenerated mock type for the Verifier type
type Verifier struct {
	mock.Mock
}

// VerifyQC provides a mock function with given fields: signers, sigData, view, blockID
func (_m *Verifier) VerifyQC(signers flow.IdentityList, sigData []byte, view uint64, blockID flow.Identifier) error {
	ret := _m.Called(signers, sigData, view, blockID)

	var r0 error
	if rf, ok := ret.Get(0).(func(flow.IdentityList, []byte, uint64, flow.Identifier) error); ok {
		r0 = rf(signers, sigData, view, blockID)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// VerifyTC provides a mock function with given fields: signers, sigData, view, highQCViews
func (_m *Verifier) VerifyTC(signers flow.IdentityList, sigData []byte, view uint64, highQCViews []uint64) error {
	ret := _m.Called(signers, sigData, view, highQCViews)

	var r0 error
	if rf, ok := ret.Get(0).(func(flow.IdentityList, []byte, uint64, []uint64) error); ok {
		r0 = rf(signers, sigData, view, highQCViews)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// VerifyVote provides a mock function with given fields: voter, sigData, view, blockID
func (_m *Verifier) VerifyVote(voter *flow.Identity, sigData []byte, view uint64, blockID flow.Identifier) error {
	ret := _m.Called(voter, sigData, view, blockID)

	var r0 error
	if rf, ok := ret.Get(0).(func(*flow.Identity, []byte, uint64, flow.Identifier) error); ok {
		r0 = rf(voter, sigData, view, blockID)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

type mockConstructorTestingTNewVerifier interface {
	mock.TestingT
	Cleanup(func())
}

// NewVerifier creates a new instance of Verifier. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
func NewVerifier(t mockConstructorTestingTNewVerifier) *Verifier {
	mock := &Verifier{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
