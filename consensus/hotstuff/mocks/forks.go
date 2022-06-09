// Code generated by mockery v2.12.3. DO NOT EDIT.

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

// AddBlock provides a mock function with given fields: block
func (_m *Forks) AddBlock(block *model.Block) error {
	ret := _m.Called(block)

	var r0 error
	if rf, ok := ret.Get(0).(func(*model.Block) error); ok {
		r0 = rf(block)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// AddQC provides a mock function with given fields: qc
func (_m *Forks) AddQC(qc *flow.QuorumCertificate) error {
	ret := _m.Called(qc)

	var r0 error
	if rf, ok := ret.Get(0).(func(*flow.QuorumCertificate) error); ok {
		r0 = rf(qc)
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

// GetBlock provides a mock function with given fields: id
func (_m *Forks) GetBlock(id flow.Identifier) (*model.Block, bool) {
	ret := _m.Called(id)

	var r0 *model.Block
	if rf, ok := ret.Get(0).(func(flow.Identifier) *model.Block); ok {
		r0 = rf(id)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*model.Block)
		}
	}

	var r1 bool
	if rf, ok := ret.Get(1).(func(flow.Identifier) bool); ok {
		r1 = rf(id)
	} else {
		r1 = ret.Get(1).(bool)
	}

	return r0, r1
}

// GetBlocksForView provides a mock function with given fields: view
func (_m *Forks) GetBlocksForView(view uint64) []*model.Block {
	ret := _m.Called(view)

	var r0 []*model.Block
	if rf, ok := ret.Get(0).(func(uint64) []*model.Block); ok {
		r0 = rf(view)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]*model.Block)
		}
	}

	return r0
}

// IsSafeBlock provides a mock function with given fields: block
func (_m *Forks) IsSafeBlock(block *model.Block) bool {
	ret := _m.Called(block)

	var r0 bool
	if rf, ok := ret.Get(0).(func(*model.Block) bool); ok {
		r0 = rf(block)
	} else {
		r0 = ret.Get(0).(bool)
	}

	return r0
}

// MakeForkChoice provides a mock function with given fields: curView
func (_m *Forks) MakeForkChoice(curView uint64) (*flow.QuorumCertificate, *model.Block, error) {
	ret := _m.Called(curView)

	var r0 *flow.QuorumCertificate
	if rf, ok := ret.Get(0).(func(uint64) *flow.QuorumCertificate); ok {
		r0 = rf(curView)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*flow.QuorumCertificate)
		}
	}

	var r1 *model.Block
	if rf, ok := ret.Get(1).(func(uint64) *model.Block); ok {
		r1 = rf(curView)
	} else {
		if ret.Get(1) != nil {
			r1 = ret.Get(1).(*model.Block)
		}
	}

	var r2 error
	if rf, ok := ret.Get(2).(func(uint64) error); ok {
		r2 = rf(curView)
	} else {
		r2 = ret.Error(2)
	}

	return r0, r1, r2
}

type NewForksT interface {
	mock.TestingT
	Cleanup(func())
}

// NewForks creates a new instance of Forks. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
func NewForks(t NewForksT) *Forks {
	mock := &Forks{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
