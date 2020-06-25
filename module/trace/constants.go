package trace

// Span names
const (
	CONProcessCollection              SpanName = "con.processCollection"
	CONHotFinalizeCollection          SpanName = "con.hotstuff.finalizeCollection"
	CONIngOnCollectionGuarantee       SpanName = "con.ingestion.onCollectionGuarantee"
	CONPropOnGuarantee                SpanName = "con.propagation.onGuarantee"
	CONCompBroadcastProposalWithDelay SpanName = "con.compliance.BroadcastProposalWithDelay"
	CONCompOnBlockProposal            SpanName = "con.compliance.onBlockProposal"
	CONProvOnBlockProposal            SpanName = "con.provider.onBlockProposal"
	CONMatchCheckSealing              SpanName = "con.matching.checkSealing"

	CONProcessBlock                SpanName = "con.processBlock"
	CONHotEventHandlerStartNewView SpanName = "con.hotstuff.eventHandler.startNewView"
	CONHotFinalizeBlock            SpanName = "con.hotstuff.finalizeBlock"

	EXEExecuteBlock           SpanName = "exe.ingestion.executeBlock"
	EXESaveExecutionResults   SpanName = "exe.ingestion.saveExecutionResults"
	EXESaveTransactionResults SpanName = "exe.ingestion.saveTransactionResults"

	EXEBroadcastExecutionReceipt SpanName = "exe.provider.broadcastExecutionReceipt"

	EXEComputeBlock       SpanName = "exe.computer.computeBlock"
	EXEComputeCollection  SpanName = "exe.computer.computeCollection"
	EXEComputeTransaction SpanName = "exe.computer.computeTransaction"

	EXECommitDelta                        SpanName = "exe.state.commitDelta"
	EXEGetRegisters                       SpanName = "exe.state.getRegisters"
	EXEGetRegistersWithProofs             SpanName = "exe.state.getRegistersWithProofs"
	EXEPersistStateCommitment             SpanName = "exe.state.persistStateCommitment"
	EXEPersistChunkDataPack               SpanName = "exe.state.persistChunkDataPack"
	EXEGetExecutionResultID               SpanName = "exe.state.getExecutionResultID"
	EXEPersistExecutionResult             SpanName = "exe.state.persistExecutionResult"
	EXEPersistStateInteractions           SpanName = "exe.state.persistStateInteractions"
	EXERetrieveStateDelta                 SpanName = "exe.state.retrieveStateDelta"
	EXEUpdateHighestExecutedBlockIfHigher SpanName = "exe.state.updateHighestExecutedBlockIfHigher"
	EXEGetHighestExecutedBlockID          SpanName = "exe.state.getHighestExecutedBlockID"
)

// Tag names
const (
	EXEParseDurationTag     = "runtime.parseTransactionDuration"
	EXECheckDurationTag     = "runtime.checkTransactionDuration"
	EXEInterpretDurationTag = "runtime.interpretTransactionDuration"
)
