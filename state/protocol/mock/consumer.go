// Code generated by mockery v1.0.0. DO NOT EDIT.

package mock

import (
	flow "github.com/onflow/flow-go/model/flow"
	mock "github.com/stretchr/testify/mock"
)

// Consumer is an autogenerated mock type for the Consumer type
type Consumer struct {
	mock.Mock
}

// BlockFinalized provides a mock function with given fields: block
func (_m *Consumer) BlockFinalized(block *flow.Header) {
	_m.Called(block)
}

// BlockProcessable provides a mock function with given fields: block
func (_m *Consumer) BlockProcessable(block *flow.Header) {
	_m.Called(block)
}

// EpochCommittedPhaseStarted provides a mock function with given fields: epoch, first
func (_m *Consumer) EpochCommittedPhaseStarted(epoch uint64, first *flow.Header) {
	_m.Called(epoch, first)
}

// EpochSetupPhaseStarted provides a mock function with given fields: epoch, first
func (_m *Consumer) EpochSetupPhaseStarted(epoch uint64, first *flow.Header) {
	_m.Called(epoch, first)
}

// EpochTransition provides a mock function with given fields: newEpoch, first
func (_m *Consumer) EpochTransition(newEpoch uint64, first *flow.Header) {
	_m.Called(newEpoch, first)
}
