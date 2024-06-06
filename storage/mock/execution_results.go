// Code generated by mockery v2.43.2. DO NOT EDIT.

package mock

import (
	flow "github.com/onflow/flow-go/model/flow"
	mock "github.com/stretchr/testify/mock"

	storage "github.com/onflow/flow-go/storage"

	transaction "github.com/onflow/flow-go/storage/badger/transaction"
)

// ExecutionResults is an autogenerated mock type for the ExecutionResults type
type ExecutionResults struct {
	mock.Mock
}

// BatchIndex provides a mock function with given fields: blockID, resultID, batch
func (_m *ExecutionResults) BatchIndex(blockID flow.Identifier, resultID flow.Identifier, batch storage.BatchStorage) error {
	ret := _m.Called(blockID, resultID, batch)

	if len(ret) == 0 {
		panic("no return value specified for BatchIndex")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(flow.Identifier, flow.Identifier, storage.BatchStorage) error); ok {
		r0 = rf(blockID, resultID, batch)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// BatchRemoveIndexByBlockID provides a mock function with given fields: blockID, batch
func (_m *ExecutionResults) BatchRemoveIndexByBlockID(blockID flow.Identifier, batch storage.BatchStorage) error {
	ret := _m.Called(blockID, batch)

	if len(ret) == 0 {
		panic("no return value specified for BatchRemoveIndexByBlockID")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(flow.Identifier, storage.BatchStorage) error); ok {
		r0 = rf(blockID, batch)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// BatchStore provides a mock function with given fields: result, batch
func (_m *ExecutionResults) BatchStore(result *flow.ExecutionResult, batch storage.BatchStorage) error {
	ret := _m.Called(result, batch)

	if len(ret) == 0 {
		panic("no return value specified for BatchStore")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(*flow.ExecutionResult, storage.BatchStorage) error); ok {
		r0 = rf(result, batch)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// ByBlockID provides a mock function with given fields: blockID
func (_m *ExecutionResults) ByBlockID(blockID flow.Identifier) (*flow.ExecutionResult, error) {
	ret := _m.Called(blockID)

	if len(ret) == 0 {
		panic("no return value specified for ByBlockID")
	}

	var r0 *flow.ExecutionResult
	var r1 error
	if rf, ok := ret.Get(0).(func(flow.Identifier) (*flow.ExecutionResult, error)); ok {
		return rf(blockID)
	}
	if rf, ok := ret.Get(0).(func(flow.Identifier) *flow.ExecutionResult); ok {
		r0 = rf(blockID)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*flow.ExecutionResult)
		}
	}

	if rf, ok := ret.Get(1).(func(flow.Identifier) error); ok {
		r1 = rf(blockID)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// ByID provides a mock function with given fields: resultID
func (_m *ExecutionResults) ByID(resultID flow.Identifier) (*flow.ExecutionResult, error) {
	ret := _m.Called(resultID)

	if len(ret) == 0 {
		panic("no return value specified for ByID")
	}

	var r0 *flow.ExecutionResult
	var r1 error
	if rf, ok := ret.Get(0).(func(flow.Identifier) (*flow.ExecutionResult, error)); ok {
		return rf(resultID)
	}
	if rf, ok := ret.Get(0).(func(flow.Identifier) *flow.ExecutionResult); ok {
		r0 = rf(resultID)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*flow.ExecutionResult)
		}
	}

	if rf, ok := ret.Get(1).(func(flow.Identifier) error); ok {
		r1 = rf(resultID)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// ByIDTx provides a mock function with given fields: resultID
func (_m *ExecutionResults) ByIDTx(resultID flow.Identifier) func(*transaction.Tx) (*flow.ExecutionResult, error) {
	ret := _m.Called(resultID)

	if len(ret) == 0 {
		panic("no return value specified for ByIDTx")
	}

	var r0 func(*transaction.Tx) (*flow.ExecutionResult, error)
	if rf, ok := ret.Get(0).(func(flow.Identifier) func(*transaction.Tx) (*flow.ExecutionResult, error)); ok {
		r0 = rf(resultID)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(func(*transaction.Tx) (*flow.ExecutionResult, error))
		}
	}

	return r0
}

// ForceIndex provides a mock function with given fields: blockID, resultID
func (_m *ExecutionResults) ForceIndex(blockID flow.Identifier, resultID flow.Identifier) error {
	ret := _m.Called(blockID, resultID)

	if len(ret) == 0 {
		panic("no return value specified for ForceIndex")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(flow.Identifier, flow.Identifier) error); ok {
		r0 = rf(blockID, resultID)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// Index provides a mock function with given fields: blockID, resultID
func (_m *ExecutionResults) Index(blockID flow.Identifier, resultID flow.Identifier) error {
	ret := _m.Called(blockID, resultID)

	if len(ret) == 0 {
		panic("no return value specified for Index")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(flow.Identifier, flow.Identifier) error); ok {
		r0 = rf(blockID, resultID)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// Store provides a mock function with given fields: result
func (_m *ExecutionResults) Store(result *flow.ExecutionResult) error {
	ret := _m.Called(result)

	if len(ret) == 0 {
		panic("no return value specified for Store")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(*flow.ExecutionResult) error); ok {
		r0 = rf(result)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// NewExecutionResults creates a new instance of ExecutionResults. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewExecutionResults(t interface {
	mock.TestingT
	Cleanup(func())
}) *ExecutionResults {
	mock := &ExecutionResults{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
