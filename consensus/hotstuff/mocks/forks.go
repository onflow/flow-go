// Code generated by mockery v2.21.4. DO NOT EDIT.

package mocks

import (
	flow "github.com/onflow/flow-go/model/flow"

	mock "github.com/stretchr/testify/mock"

	model "github.com/onflow/flow-go/consensus/hotstuff/model"
)

// Forks is an autogenerated mock type for the Forks type
type Forks struct {
	mock.Mock
}

// AddProposal provides a mock function with given fields: proposal
func (_m *Forks) AddProposal(proposal *model.Proposal) error {
	ret := _m.Called(proposal)

	var r0 error
	if rf, ok := ret.Get(0).(func(*model.Proposal) error); ok {
		r0 = rf(proposal)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// FinalizedBlock provides a mock function with given fields:
func (_m *Forks) FinalizedBlock() *model.Block {
	ret := _m.Called()

	var r0 *model.Block
	if rf, ok := ret.Get(0).(func() *model.Block); ok {
		r0 = rf()
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*model.Block)
		}
	}

	return r0
}

// FinalizedView provides a mock function with given fields:
func (_m *Forks) FinalizedView() uint64 {
	ret := _m.Called()

	var r0 uint64
	if rf, ok := ret.Get(0).(func() uint64); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(uint64)
	}

	return r0
}

// GetProposal provides a mock function with given fields: id
func (_m *Forks) GetProposal(id flow.Identifier) (*model.Proposal, bool) {
	ret := _m.Called(id)

	var r0 *model.Proposal
	var r1 bool
	if rf, ok := ret.Get(0).(func(flow.Identifier) (*model.Proposal, bool)); ok {
		return rf(id)
	}
	if rf, ok := ret.Get(0).(func(flow.Identifier) *model.Proposal); ok {
		r0 = rf(id)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*model.Proposal)
		}
	}

	if rf, ok := ret.Get(1).(func(flow.Identifier) bool); ok {
		r1 = rf(id)
	} else {
		r1 = ret.Get(1).(bool)
	}

	return r0, r1
}

// GetProposalsForView provides a mock function with given fields: view
func (_m *Forks) GetProposalsForView(view uint64) []*model.Proposal {
	ret := _m.Called(view)

	var r0 []*model.Proposal
	if rf, ok := ret.Get(0).(func(uint64) []*model.Proposal); ok {
		r0 = rf(view)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]*model.Proposal)
		}
	}

	return r0
}

// NewestView provides a mock function with given fields:
func (_m *Forks) NewestView() uint64 {
	ret := _m.Called()

	var r0 uint64
	if rf, ok := ret.Get(0).(func() uint64); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(uint64)
	}

	return r0
}

type mockConstructorTestingTNewForks interface {
	mock.TestingT
	Cleanup(func())
}

// NewForks creates a new instance of Forks. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
func NewForks(t mockConstructorTestingTNewForks) *Forks {
	mock := &Forks{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
