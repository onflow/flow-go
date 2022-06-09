// Code generated by mockery v2.12.3. DO NOT EDIT.

package mempool

import (
	flow "github.com/onflow/flow-go/model/flow"

	mock "github.com/stretchr/testify/mock"
)

// Blocks is an autogenerated mock type for the Blocks type
type Blocks struct {
	mock.Mock
}

// Add provides a mock function with given fields: block
func (_m *Blocks) Add(block *flow.Block) bool {
	ret := _m.Called(block)

	var r0 bool
	if rf, ok := ret.Get(0).(func(*flow.Block) bool); ok {
		r0 = rf(block)
	} else {
		r0 = ret.Get(0).(bool)
	}

	return r0
}

// All provides a mock function with given fields:
func (_m *Blocks) All() []*flow.Block {
	ret := _m.Called()

	var r0 []*flow.Block
	if rf, ok := ret.Get(0).(func() []*flow.Block); ok {
		r0 = rf()
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]*flow.Block)
		}
	}

	return r0
}

// ByID provides a mock function with given fields: blockID
func (_m *Blocks) ByID(blockID flow.Identifier) (*flow.Block, bool) {
	ret := _m.Called(blockID)

	var r0 *flow.Block
	if rf, ok := ret.Get(0).(func(flow.Identifier) *flow.Block); ok {
		r0 = rf(blockID)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*flow.Block)
		}
	}

	var r1 bool
	if rf, ok := ret.Get(1).(func(flow.Identifier) bool); ok {
		r1 = rf(blockID)
	} else {
		r1 = ret.Get(1).(bool)
	}

	return r0, r1
}

// Has provides a mock function with given fields: blockID
func (_m *Blocks) Has(blockID flow.Identifier) bool {
	ret := _m.Called(blockID)

	var r0 bool
	if rf, ok := ret.Get(0).(func(flow.Identifier) bool); ok {
		r0 = rf(blockID)
	} else {
		r0 = ret.Get(0).(bool)
	}

	return r0
}

// Hash provides a mock function with given fields:
func (_m *Blocks) Hash() flow.Identifier {
	ret := _m.Called()

	var r0 flow.Identifier
	if rf, ok := ret.Get(0).(func() flow.Identifier); ok {
		r0 = rf()
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(flow.Identifier)
		}
	}

	return r0
}

// Rem provides a mock function with given fields: blockID
func (_m *Blocks) Rem(blockID flow.Identifier) bool {
	ret := _m.Called(blockID)

	var r0 bool
	if rf, ok := ret.Get(0).(func(flow.Identifier) bool); ok {
		r0 = rf(blockID)
	} else {
		r0 = ret.Get(0).(bool)
	}

	return r0
}

// Size provides a mock function with given fields:
func (_m *Blocks) Size() uint {
	ret := _m.Called()

	var r0 uint
	if rf, ok := ret.Get(0).(func() uint); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(uint)
	}

	return r0
}

type NewBlocksT interface {
	mock.TestingT
	Cleanup(func())
}

// NewBlocks creates a new instance of Blocks. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
func NewBlocks(t NewBlocksT) *Blocks {
	mock := &Blocks{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
