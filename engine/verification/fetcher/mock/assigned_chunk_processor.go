// Code generated by mockery v1.0.0. DO NOT EDIT.

package mockfetcher

import (
	chunks "github.com/onflow/flow-go/model/chunks"

	mock "github.com/stretchr/testify/mock"

	module "github.com/onflow/flow-go/module"
)

// AssignedChunkProcessor is an autogenerated mock type for the AssignedChunkProcessor type
type AssignedChunkProcessor struct {
	mock.Mock
}

// ProcessAssignedChunk provides a mock function with given fields: locator
func (_m *AssignedChunkProcessor) ProcessAssignedChunk(locator *chunks.Locator) {
	_m.Called(locator)
}

// WithChunkConsumerNotifier provides a mock function with given fields: notifier
func (_m *AssignedChunkProcessor) WithChunkConsumerNotifier(notifier module.ProcessingNotifier) {
	_m.Called(notifier)
}
