// Code generated by mockery v2.53.3. DO NOT EDIT.

package mock

import (
	fvm "github.com/onflow/flow-go/fvm"
	mock "github.com/stretchr/testify/mock"

	snapshot "github.com/onflow/flow-go/fvm/storage/snapshot"

	storage "github.com/onflow/flow-go/fvm/storage"
)

// VM is an autogenerated mock type for the VM type
type VM struct {
	mock.Mock
}

// NewExecutor provides a mock function with given fields: _a0, _a1, _a2
func (_m *VM) NewExecutor(_a0 fvm.Context, _a1 fvm.Procedure, _a2 storage.TransactionPreparer) fvm.ProcedureExecutor {
	ret := _m.Called(_a0, _a1, _a2)

	if len(ret) == 0 {
		panic("no return value specified for NewExecutor")
	}

	var r0 fvm.ProcedureExecutor
	if rf, ok := ret.Get(0).(func(fvm.Context, fvm.Procedure, storage.TransactionPreparer) fvm.ProcedureExecutor); ok {
		r0 = rf(_a0, _a1, _a2)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(fvm.ProcedureExecutor)
		}
	}

	return r0
}

// Run provides a mock function with given fields: _a0, _a1, _a2
func (_m *VM) Run(_a0 fvm.Context, _a1 fvm.Procedure, _a2 snapshot.StorageSnapshot) (*snapshot.ExecutionSnapshot, fvm.ProcedureOutput, error) {
	ret := _m.Called(_a0, _a1, _a2)

	if len(ret) == 0 {
		panic("no return value specified for Run")
	}

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

// NewVM creates a new instance of VM. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewVM(t interface {
	mock.TestingT
	Cleanup(func())
}) *VM {
	mock := &VM{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
