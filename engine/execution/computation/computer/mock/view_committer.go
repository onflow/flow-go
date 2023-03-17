// Code generated by mockery v2.21.4. DO NOT EDIT.

package mock

import (
	ledger "github.com/onflow/flow-go/ledger"
	flow "github.com/onflow/flow-go/model/flow"

	mock "github.com/stretchr/testify/mock"

	state "github.com/onflow/flow-go/fvm/state"
)

// ViewCommitter is an autogenerated mock type for the ViewCommitter type
type ViewCommitter struct {
	mock.Mock
}

// CommitView provides a mock function with given fields: _a0, _a1
func (_m *ViewCommitter) CommitView(_a0 *state.ExecutionSnapshot, _a1 flow.StateCommitment) (flow.StateCommitment, []byte, *ledger.TrieUpdate, error) {
	ret := _m.Called(_a0, _a1)

	var r0 flow.StateCommitment
	var r1 []byte
	var r2 *ledger.TrieUpdate
	var r3 error
	if rf, ok := ret.Get(0).(func(*state.ExecutionSnapshot, flow.StateCommitment) (flow.StateCommitment, []byte, *ledger.TrieUpdate, error)); ok {
		return rf(_a0, _a1)
	}
	if rf, ok := ret.Get(0).(func(*state.ExecutionSnapshot, flow.StateCommitment) flow.StateCommitment); ok {
		r0 = rf(_a0, _a1)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(flow.StateCommitment)
		}
	}

	if rf, ok := ret.Get(1).(func(*state.ExecutionSnapshot, flow.StateCommitment) []byte); ok {
		r1 = rf(_a0, _a1)
	} else {
		if ret.Get(1) != nil {
			r1 = ret.Get(1).([]byte)
		}
	}

	if rf, ok := ret.Get(2).(func(*state.ExecutionSnapshot, flow.StateCommitment) *ledger.TrieUpdate); ok {
		r2 = rf(_a0, _a1)
	} else {
		if ret.Get(2) != nil {
			r2 = ret.Get(2).(*ledger.TrieUpdate)
		}
	}

	if rf, ok := ret.Get(3).(func(*state.ExecutionSnapshot, flow.StateCommitment) error); ok {
		r3 = rf(_a0, _a1)
	} else {
		r3 = ret.Error(3)
	}

	return r0, r1, r2, r3
}

type mockConstructorTestingTNewViewCommitter interface {
	mock.TestingT
	Cleanup(func())
}

// NewViewCommitter creates a new instance of ViewCommitter. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
func NewViewCommitter(t mockConstructorTestingTNewViewCommitter) *ViewCommitter {
	mock := &ViewCommitter{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
