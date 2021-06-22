// Code generated by mockery v1.0.0. DO NOT EDIT.

package mock

import mock "github.com/stretchr/testify/mock"

// VerificationMetrics is an autogenerated mock type for the VerificationMetrics type
type VerificationMetrics struct {
	mock.Mock
}

// OnAssignedChunkProcessedAtAssigner provides a mock function with given fields:
func (_m *VerificationMetrics) OnAssignedChunkProcessedAtAssigner() {
	_m.Called()
}

// OnAssignedChunkReceivedAtFetcher provides a mock function with given fields:
func (_m *VerificationMetrics) OnAssignedChunkReceivedAtFetcher() {
	_m.Called()
}

// OnBlockConsumerJobDone provides a mock function with given fields: _a0
func (_m *VerificationMetrics) OnBlockConsumerJobDone(_a0 uint64) {
	_m.Called(_a0)
}

// OnChunkConsumerJobDone provides a mock function with given fields: _a0
func (_m *VerificationMetrics) OnChunkConsumerJobDone(_a0 uint64) {
	_m.Called(_a0)
}

// OnChunkDataPackArrivedAtFetcher provides a mock function with given fields:
func (_m *VerificationMetrics) OnChunkDataPackArrivedAtFetcher() {
	_m.Called()
}

// OnChunkDataPackReceived provides a mock function with given fields:
func (_m *VerificationMetrics) OnChunkDataPackReceived() {
	_m.Called()
}

// OnChunkDataPackRequestDispatchedInNetwork provides a mock function with given fields:
func (_m *VerificationMetrics) OnChunkDataPackRequestDispatchedInNetworkByRequester() {
	_m.Called()
}

// OnChunkDataPackRequestReceivedByRequester provides a mock function with given fields:
func (_m *VerificationMetrics) OnChunkDataPackRequestReceivedByRequester() {
	_m.Called()
}

// OnChunkDataPackRequestSentByFetcher provides a mock function with given fields:
func (_m *VerificationMetrics) OnChunkDataPackRequestSentByFetcher() {
	_m.Called()
}

// OnChunkDataPackRequested provides a mock function with given fields:
func (_m *VerificationMetrics) OnChunkDataPackRequested() {
	_m.Called()
}

// OnChunkDataPackResponseReceivedFromNetwork provides a mock function with given fields:
func (_m *VerificationMetrics) OnChunkDataPackResponseReceivedFromNetworkByRequester() {
	_m.Called()
}

// OnChunkDataPackSentToFetcher provides a mock function with given fields:
func (_m *VerificationMetrics) OnChunkDataPackSentToFetcher() {
	_m.Called()
}

// OnChunksAssignmentDoneAtAssigner provides a mock function with given fields: chunks
func (_m *VerificationMetrics) OnChunksAssignmentDoneAtAssigner(chunks int) {
	_m.Called(chunks)
}

// OnExecutionReceiptReceived provides a mock function with given fields:
func (_m *VerificationMetrics) OnExecutionResultReceivedAtAssignerEngine() {
	_m.Called()
}

// OnExecutionResultReceived provides a mock function with given fields:
func (_m *VerificationMetrics) OnExecutionResultReceived() {
	_m.Called()
}

// OnExecutionResultSent provides a mock function with given fields:
func (_m *VerificationMetrics) OnExecutionResultSent() {
	_m.Called()
}

// OnFinalizedBlockArrivedAtAssigner provides a mock function with given fields: height
func (_m *VerificationMetrics) OnFinalizedBlockArrivedAtAssigner(height uint64) {
	_m.Called(height)
}

// OnResultApprovalDispatchedInNetwork provides a mock function with given fields:
func (_m *VerificationMetrics) OnResultApprovalDispatchedInNetworkByVerifier() {
	_m.Called()
}

// OnVerifiableChunkReceivedAtVerifierEngine provides a mock function with given fields:
func (_m *VerificationMetrics) OnVerifiableChunkReceivedAtVerifierEngine() {
	_m.Called()
}

// OnVerifiableChunkSent provides a mock function with given fields:
func (_m *VerificationMetrics) OnVerifiableChunkSent() {
	_m.Called()
}

// OnVerifiableChunkSentToVerifier provides a mock function with given fields:
func (_m *VerificationMetrics) OnVerifiableChunkSentToVerifier() {
	_m.Called()
}
