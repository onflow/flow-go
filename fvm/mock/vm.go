// Code generated by mockery v2.21.4. DO NOT EDIT.

package mock

import (
	fvm "github.com/onflow/flow-go/fvm"
	flow "github.com/onflow/flow-go/model/flow"

	mock "github.com/stretchr/testify/mock"

	snapshot "github.com/onflow/flow-go/fvm/storage/snapshot"
)

// VM is an autogenerated mock type for the VM type
type VM struct {
	mock.Mock
}

// GetAccount provides a mock function with given fields: _a0, _a1, _a2
func (_m *VM) GetAccount(_a0 fvm.Context, _a1 flow.Address, _a2 snapshot.StorageSnapshot) (*flow.Account, error) {
	ret := _m.Called(_a0, _a1, _a2)

	var r0 *flow.Account
	var r1 error
	if rf, ok := ret.Get(0).(func(fvm.Context, flow.Address, snapshot.StorageSnapshot) (*flow.Account, error)); ok {
		return rf(_a0, _a1, _a2)
	}
	if rf, ok := ret.Get(0).(func(fvm.Context, flow.Address, snapshot.StorageSnapshot) *flow.Account); ok {
		r0 = rf(_a0, _a1, _a2)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*flow.Account)
		}
	}

	if rf, ok := ret.Get(1).(func(fvm.Context, flow.Address, snapshot.StorageSnapshot) error); ok {
		r1 = rf(_a0, _a1, _a2)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// Run provides a mock function with given fields: _a0, _a1, _a2
func (_m *VM) Run(_a0 fvm.Context, _a1 fvm.Procedure, _a2 snapshot.StorageSnapshot) (*snapshot.ExecutionSnapshot, fvm.ProcedureOutput, error) {
	ret := _m.Called(_a0, _a1, _a2)

	var r0 *snapshot.ExecutionSnapshot
	var r1 fvm.ProcedureOutput
	var r2 error
	if rf, ok := ret.Get(0).(func(fvm.Context, fvm.Procedure, snapshot.StorageSnapshot) (*snapshot.ExecutionSnapshot, fvm.ProcedureOutput, error)); ok {
		return rf(_a0, _a1, _a2)
	}
	if rf, ok := ret.Get(0).(func(fvm.Context, fvm.Procedure, snapshot.StorageSnapshot) *snapshot.ExecutionSnapshot); ok {
		r0 = rf(_a0, _a1, _a2)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*snapshot.ExecutionSnapshot)
		}
	}

	if rf, ok := ret.Get(1).(func(fvm.Context, fvm.Procedure, snapshot.StorageSnapshot) fvm.ProcedureOutput); ok {
		r1 = rf(_a0, _a1, _a2)
	} else {
		r1 = ret.Get(1).(fvm.ProcedureOutput)
	}

	if rf, ok := ret.Get(2).(func(fvm.Context, fvm.Procedure, snapshot.StorageSnapshot) error); ok {
		r2 = rf(_a0, _a1, _a2)
	} else {
		r2 = ret.Error(2)
	}

	return r0, r1, r2
}

type mockConstructorTestingTNewVM interface {
	mock.TestingT
	Cleanup(func())
}

// NewVM creates a new instance of VM. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
func NewVM(t mockConstructorTestingTNewVM) *VM {
	mock := &VM{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
