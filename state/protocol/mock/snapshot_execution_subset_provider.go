// Code generated by mockery v2.53.3. DO NOT EDIT.

package mock

import (
	flow "github.com/onflow/flow-go/model/flow"
	mock "github.com/stretchr/testify/mock"

	protocol "github.com/onflow/flow-go/state/protocol"
)

// SnapshotExecutionSubsetProvider is an autogenerated mock type for the SnapshotExecutionSubsetProvider type
type SnapshotExecutionSubsetProvider struct {
	mock.Mock
}

// AtBlockID provides a mock function with given fields: blockID
func (_m *SnapshotExecutionSubsetProvider) AtBlockID(blockID flow.Identifier) protocol.SnapshotExecutionSubset {
	ret := _m.Called(blockID)

	if len(ret) == 0 {
		panic("no return value specified for AtBlockID")
	}

	var r0 protocol.SnapshotExecutionSubset
	if rf, ok := ret.Get(0).(func(flow.Identifier) protocol.SnapshotExecutionSubset); ok {
		r0 = rf(blockID)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(protocol.SnapshotExecutionSubset)
		}
	}

	return r0
}

// NewSnapshotExecutionSubsetProvider creates a new instance of SnapshotExecutionSubsetProvider. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewSnapshotExecutionSubsetProvider(t interface {
	mock.TestingT
	Cleanup(func())
}) *SnapshotExecutionSubsetProvider {
	mock := &SnapshotExecutionSubsetProvider{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
