// Code generated by mockery v2.53.3. DO NOT EDIT.

package mock

import (
	flow "github.com/onflow/flow-go/model/flow"
	mock "github.com/stretchr/testify/mock"

	module "github.com/onflow/flow-go/module"

	time "time"
)

// ExecutionMetrics is an autogenerated mock type for the ExecutionMetrics type
type ExecutionMetrics struct {
	mock.Mock
}

// ChunkDataPackRequestProcessed provides a mock function with no fields
func (_m *ExecutionMetrics) ChunkDataPackRequestProcessed() {
	_m.Called()
}

// EVMBlockExecuted provides a mock function with given fields: txCount, totalGasUsed, totalSupplyInFlow
func (_m *ExecutionMetrics) EVMBlockExecuted(txCount int, totalGasUsed uint64, totalSupplyInFlow float64) {
	_m.Called(txCount, totalGasUsed, totalSupplyInFlow)
}

// EVMTransactionExecuted provides a mock function with given fields: gasUsed, isDirectCall, failed
func (_m *ExecutionMetrics) EVMTransactionExecuted(gasUsed uint64, isDirectCall bool, failed bool) {
	_m.Called(gasUsed, isDirectCall, failed)
}

// ExecutionBlockCachedPrograms provides a mock function with given fields: programs
func (_m *ExecutionMetrics) ExecutionBlockCachedPrograms(programs int) {
	_m.Called(programs)
}

// ExecutionBlockDataUploadFinished provides a mock function with given fields: dur
func (_m *ExecutionMetrics) ExecutionBlockDataUploadFinished(dur time.Duration) {
	_m.Called(dur)
}

// ExecutionBlockDataUploadStarted provides a mock function with no fields
func (_m *ExecutionMetrics) ExecutionBlockDataUploadStarted() {
	_m.Called()
}

// ExecutionBlockExecuted provides a mock function with given fields: dur, stats
func (_m *ExecutionMetrics) ExecutionBlockExecuted(dur time.Duration, stats module.BlockExecutionResultStats) {
	_m.Called(dur, stats)
}

// ExecutionBlockExecutionEffortVectorComponent provides a mock function with given fields: _a0, _a1
func (_m *ExecutionMetrics) ExecutionBlockExecutionEffortVectorComponent(_a0 string, _a1 uint64) {
	_m.Called(_a0, _a1)
}

// ExecutionCheckpointSize provides a mock function with given fields: bytes
func (_m *ExecutionMetrics) ExecutionCheckpointSize(bytes uint64) {
	_m.Called(bytes)
}

// ExecutionChunkDataPackGenerated provides a mock function with given fields: proofSize, numberOfTransactions
func (_m *ExecutionMetrics) ExecutionChunkDataPackGenerated(proofSize int, numberOfTransactions int) {
	_m.Called(proofSize, numberOfTransactions)
}

// ExecutionCollectionExecuted provides a mock function with given fields: dur, stats
func (_m *ExecutionMetrics) ExecutionCollectionExecuted(dur time.Duration, stats module.CollectionExecutionResultStats) {
	_m.Called(dur, stats)
}

// ExecutionCollectionRequestSent provides a mock function with no fields
func (_m *ExecutionMetrics) ExecutionCollectionRequestSent() {
	_m.Called()
}

// ExecutionComputationResultUploadRetried provides a mock function with no fields
func (_m *ExecutionMetrics) ExecutionComputationResultUploadRetried() {
	_m.Called()
}

// ExecutionComputationResultUploaded provides a mock function with no fields
func (_m *ExecutionMetrics) ExecutionComputationResultUploaded() {
	_m.Called()
}

// ExecutionLastChunkDataPackPrunedHeight provides a mock function with given fields: height
func (_m *ExecutionMetrics) ExecutionLastChunkDataPackPrunedHeight(height uint64) {
	_m.Called(height)
}

// ExecutionLastExecutedBlockHeight provides a mock function with given fields: height
func (_m *ExecutionMetrics) ExecutionLastExecutedBlockHeight(height uint64) {
	_m.Called(height)
}

// ExecutionLastFinalizedExecutedBlockHeight provides a mock function with given fields: height
func (_m *ExecutionMetrics) ExecutionLastFinalizedExecutedBlockHeight(height uint64) {
	_m.Called(height)
}

// ExecutionScriptExecuted provides a mock function with given fields: dur, compUsed, memoryUsed, memoryEstimate
func (_m *ExecutionMetrics) ExecutionScriptExecuted(dur time.Duration, compUsed uint64, memoryUsed uint64, memoryEstimate uint64) {
	_m.Called(dur, compUsed, memoryUsed, memoryEstimate)
}

// ExecutionStorageStateCommitment provides a mock function with given fields: bytes
func (_m *ExecutionMetrics) ExecutionStorageStateCommitment(bytes int64) {
	_m.Called(bytes)
}

// ExecutionSync provides a mock function with given fields: syncing
func (_m *ExecutionMetrics) ExecutionSync(syncing bool) {
	_m.Called(syncing)
}

// ExecutionTargetChunkDataPackPrunedHeight provides a mock function with given fields: height
func (_m *ExecutionMetrics) ExecutionTargetChunkDataPackPrunedHeight(height uint64) {
	_m.Called(height)
}

// ExecutionTransactionExecuted provides a mock function with given fields: dur, stats, info
func (_m *ExecutionMetrics) ExecutionTransactionExecuted(dur time.Duration, stats module.TransactionExecutionResultStats, info module.TransactionExecutionResultInfo) {
	_m.Called(dur, stats, info)
}

// FinishBlockReceivedToExecuted provides a mock function with given fields: blockID
func (_m *ExecutionMetrics) FinishBlockReceivedToExecuted(blockID flow.Identifier) {
	_m.Called(blockID)
}

// ForestApproxMemorySize provides a mock function with given fields: bytes
func (_m *ExecutionMetrics) ForestApproxMemorySize(bytes uint64) {
	_m.Called(bytes)
}

// ForestNumberOfTrees provides a mock function with given fields: number
func (_m *ExecutionMetrics) ForestNumberOfTrees(number uint64) {
	_m.Called(number)
}

// LatestTrieMaxDepthTouched provides a mock function with given fields: maxDepth
func (_m *ExecutionMetrics) LatestTrieMaxDepthTouched(maxDepth uint16) {
	_m.Called(maxDepth)
}

// LatestTrieRegCount provides a mock function with given fields: number
func (_m *ExecutionMetrics) LatestTrieRegCount(number uint64) {
	_m.Called(number)
}

// LatestTrieRegCountDiff provides a mock function with given fields: number
func (_m *ExecutionMetrics) LatestTrieRegCountDiff(number int64) {
	_m.Called(number)
}

// LatestTrieRegSize provides a mock function with given fields: size
func (_m *ExecutionMetrics) LatestTrieRegSize(size uint64) {
	_m.Called(size)
}

// LatestTrieRegSizeDiff provides a mock function with given fields: size
func (_m *ExecutionMetrics) LatestTrieRegSizeDiff(size int64) {
	_m.Called(size)
}

// ProofSize provides a mock function with given fields: bytes
func (_m *ExecutionMetrics) ProofSize(bytes uint32) {
	_m.Called(bytes)
}

// ReadDuration provides a mock function with given fields: duration
func (_m *ExecutionMetrics) ReadDuration(duration time.Duration) {
	_m.Called(duration)
}

// ReadDurationPerItem provides a mock function with given fields: duration
func (_m *ExecutionMetrics) ReadDurationPerItem(duration time.Duration) {
	_m.Called(duration)
}

// ReadValuesNumber provides a mock function with given fields: number
func (_m *ExecutionMetrics) ReadValuesNumber(number uint64) {
	_m.Called(number)
}

// ReadValuesSize provides a mock function with given fields: byte
func (_m *ExecutionMetrics) ReadValuesSize(byte uint64) {
	_m.Called(byte)
}

// RuntimeSetNumberOfAccounts provides a mock function with given fields: count
func (_m *ExecutionMetrics) RuntimeSetNumberOfAccounts(count uint64) {
	_m.Called(count)
}

// RuntimeTransactionChecked provides a mock function with given fields: dur
func (_m *ExecutionMetrics) RuntimeTransactionChecked(dur time.Duration) {
	_m.Called(dur)
}

// RuntimeTransactionInterpreted provides a mock function with given fields: dur
func (_m *ExecutionMetrics) RuntimeTransactionInterpreted(dur time.Duration) {
	_m.Called(dur)
}

// RuntimeTransactionParsed provides a mock function with given fields: dur
func (_m *ExecutionMetrics) RuntimeTransactionParsed(dur time.Duration) {
	_m.Called(dur)
}

// RuntimeTransactionProgramsCacheHit provides a mock function with no fields
func (_m *ExecutionMetrics) RuntimeTransactionProgramsCacheHit() {
	_m.Called()
}

// RuntimeTransactionProgramsCacheMiss provides a mock function with no fields
func (_m *ExecutionMetrics) RuntimeTransactionProgramsCacheMiss() {
	_m.Called()
}

// SetNumberOfDeployedCOAs provides a mock function with given fields: count
func (_m *ExecutionMetrics) SetNumberOfDeployedCOAs(count uint64) {
	_m.Called(count)
}

// StartBlockReceivedToExecuted provides a mock function with given fields: blockID
func (_m *ExecutionMetrics) StartBlockReceivedToExecuted(blockID flow.Identifier) {
	_m.Called(blockID)
}

// UpdateCollectionMaxHeight provides a mock function with given fields: height
func (_m *ExecutionMetrics) UpdateCollectionMaxHeight(height uint64) {
	_m.Called(height)
}

// UpdateCount provides a mock function with no fields
func (_m *ExecutionMetrics) UpdateCount() {
	_m.Called()
}

// UpdateDuration provides a mock function with given fields: duration
func (_m *ExecutionMetrics) UpdateDuration(duration time.Duration) {
	_m.Called(duration)
}

// UpdateDurationPerItem provides a mock function with given fields: duration
func (_m *ExecutionMetrics) UpdateDurationPerItem(duration time.Duration) {
	_m.Called(duration)
}

// UpdateValuesNumber provides a mock function with given fields: number
func (_m *ExecutionMetrics) UpdateValuesNumber(number uint64) {
	_m.Called(number)
}

// UpdateValuesSize provides a mock function with given fields: byte
func (_m *ExecutionMetrics) UpdateValuesSize(byte uint64) {
	_m.Called(byte)
}

// NewExecutionMetrics creates a new instance of ExecutionMetrics. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewExecutionMetrics(t interface {
	mock.TestingT
	Cleanup(func())
}) *ExecutionMetrics {
	mock := &ExecutionMetrics{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
