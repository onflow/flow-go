package trace

const (
	EXEExecuteBlock           = "ingestion.executeBlock"
	EXESaveExecutionResults   = "ingestion.saveExecutionResults"
	EXESaveTransactionResults = "ingestion.saveTransactionResults"

	EXEBroadcastExecutionReceipt = "provider.broadcastExecutionReceipt"

	EXEComputeBlock       = "computer.computeBlock"
	EXEComputeCollection  = "computer.computeCollection"
	EXEComputeTransaction = "computer.computeTransaction"

	EXECommitDelta                        = "state.commitDelta"
	EXEGetRegisters                       = "state.getRegisters"
	EXEGetRegistersWithProofs             = "state.getRegistersWithProofs"
	EXEPersistStateCommitment             = "state.persistStateCommitment"
	EXEPersistChunkDataPack               = "state.persistChunkDataPack"
	EXEGetExecutionResultID               = "state.getExecutionResultID"
	EXEPersistExecutionResult             = "state.persistExecutionResult"
	EXEPersistStateInteractions           = "state.persistStateInteractions"
	EXERetrieveStateDelta                 = "state.retrieveStateDelta"
	EXEUpdateHighestExecutedBlockIfHigher = "state.updateHighestExecutedBlockIfHigher"
	EXEGetHighestExecutedBlockID          = "state.getHighestExecutedBlockID"
)
