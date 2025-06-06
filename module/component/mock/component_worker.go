// Code generated by mockery v2.53.3. DO NOT EDIT.

package component

import (
	component "github.com/onflow/flow-go/module/component"
	irrecoverable "github.com/onflow/flow-go/module/irrecoverable"

	mock "github.com/stretchr/testify/mock"
)

// ComponentWorker is an autogenerated mock type for the ComponentWorker type
type ComponentWorker struct {
	mock.Mock
}

// Execute provides a mock function with given fields: ctx, ready
func (_m *ComponentWorker) Execute(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
	_m.Called(ctx, ready)
}

// NewComponentWorker creates a new instance of ComponentWorker. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewComponentWorker(t interface {
	mock.TestingT
	Cleanup(func())
}) *ComponentWorker {
	mock := &ComponentWorker{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
