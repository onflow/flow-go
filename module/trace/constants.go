package trace

// Span names
const (
	EXEExecuteBlock           SpanName = "ingestion.executeBlock"
	EXESaveExecutionResults   SpanName = "ingestion.saveExecutionResults"
	EXESaveTransactionResults SpanName = "ingestion.saveTransactionResults"

	EXEBroadcastExecutionReceipt SpanName = "provider.broadcastExecutionReceipt"

	EXEComputeBlock       SpanName = "computer.computeBlock"
	EXEComputeCollection  SpanName = "computer.computeCollection"
	EXEComputeTransaction SpanName = "computer.computeTransaction"

	EXECommitDelta                        SpanName = "state.commitDelta"
	EXEGetRegisters                       SpanName = "state.getRegisters"
	EXEGetRegistersWithProofs             SpanName = "state.getRegistersWithProofs"
	EXEPersistStateCommitment             SpanName = "state.persistStateCommitment"
	EXEPersistChunkDataPack               SpanName = "state.persistChunkDataPack"
	EXEGetExecutionResultID               SpanName = "state.getExecutionResultID"
	EXEPersistExecutionResult             SpanName = "state.persistExecutionResult"
	EXEPersistStateInteractions           SpanName = "state.persistStateInteractions"
	EXERetrieveStateDelta                 SpanName = "state.retrieveStateDelta"
	EXEUpdateHighestExecutedBlockIfHigher SpanName = "state.updateHighestExecutedBlockIfHigher"
	EXEGetHighestExecutedBlockID          SpanName = "state.getHighestExecutedBlockID"
)

// Tag names
const (
	EXEParseDurationTag     = "runtime.parseTransactionDuration"
	EXECheckDurationTag     = "runtime.checkTransactionDuration"
	EXEInterpretDurationTag = "runtime.interpretTransactionDuration"
)
