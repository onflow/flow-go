// Code generated by mockery v2.53.3. DO NOT EDIT.

package mocks

import (
	flow "github.com/onflow/flow-go/model/flow"

	mock "github.com/stretchr/testify/mock"

	model "github.com/onflow/flow-go/consensus/hotstuff/model"
)

// Signer is an autogenerated mock type for the Signer type
type Signer struct {
	mock.Mock
}

// CreateTimeout provides a mock function with given fields: curView, newestQC, lastViewTC
func (_m *Signer) CreateTimeout(curView uint64, newestQC *flow.QuorumCertificate, lastViewTC *flow.TimeoutCertificate) (*model.TimeoutObject, error) {
	ret := _m.Called(curView, newestQC, lastViewTC)

	if len(ret) == 0 {
		panic("no return value specified for CreateTimeout")
	}

	var r0 *model.TimeoutObject
	var r1 error
	if rf, ok := ret.Get(0).(func(uint64, *flow.QuorumCertificate, *flow.TimeoutCertificate) (*model.TimeoutObject, error)); ok {
		return rf(curView, newestQC, lastViewTC)
	}
	if rf, ok := ret.Get(0).(func(uint64, *flow.QuorumCertificate, *flow.TimeoutCertificate) *model.TimeoutObject); ok {
		r0 = rf(curView, newestQC, lastViewTC)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*model.TimeoutObject)
		}
	}

	if rf, ok := ret.Get(1).(func(uint64, *flow.QuorumCertificate, *flow.TimeoutCertificate) error); ok {
		r1 = rf(curView, newestQC, lastViewTC)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// CreateVote provides a mock function with given fields: block
func (_m *Signer) CreateVote(block *model.Block) (*model.Vote, error) {
	ret := _m.Called(block)

	if len(ret) == 0 {
		panic("no return value specified for CreateVote")
	}

	var r0 *model.Vote
	var r1 error
	if rf, ok := ret.Get(0).(func(*model.Block) (*model.Vote, error)); ok {
		return rf(block)
	}
	if rf, ok := ret.Get(0).(func(*model.Block) *model.Vote); ok {
		r0 = rf(block)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*model.Vote)
		}
	}

	if rf, ok := ret.Get(1).(func(*model.Block) error); ok {
		r1 = rf(block)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// NewSigner creates a new instance of Signer. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewSigner(t interface {
	mock.TestingT
	Cleanup(func())
}) *Signer {
	mock := &Signer{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
