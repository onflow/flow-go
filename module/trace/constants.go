package trace

// Span names
const (
	// Consensus Node

	CONProcessCollection SpanName = "con.processCollection"
	// children of CONProcessCollection
	CONHotFinalizeCollection          SpanName = "con.hotstuff.finalizeCollection"
	CONIngOnCollectionGuarantee       SpanName = "con.ingestion.onCollectionGuarantee"
	CONPropOnGuarantee                SpanName = "con.propagation.onGuarantee"
	CONCompBroadcastProposalWithDelay SpanName = "con.compliance.BroadcastProposalWithDelay"
	CONCompOnBlockProposal            SpanName = "con.compliance.onBlockProposal"
	CONProvOnBlockProposal            SpanName = "con.provider.onBlockProposal"
	CONMatchCheckSealing              SpanName = "con.matching.checkSealing"

	CONProcessBlock SpanName = "con.processBlock"
	// children of CONProcessBlock
	CONHotEventHandlerStartNewView SpanName = "con.hotstuff.eventHandler.startNewView"
	CONHotFinalizeBlock            SpanName = "con.hotstuff.finalizeBlock"

	// Execution Node

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

	// Verification node

	VERProcessExecutionReceipt SpanName = "ver.processExecutionReceipt"
	// children of VERProcessExecutionReceipt
	VERFindHandleExecutionReceipt SpanName = "ver.find.handleExecutionReceipt"
	VERFindOnFinalizedBlock       SpanName = "ver.finder.OnFinalizedBlock"
	VERFindCheckCachedReceipts    SpanName = "ver.finder.checkCachedReceipts"
	VERFindCheckPendingReceipts   SpanName = "ver.finder.checkPendingReceipts"
	VERFindCheckReadyReceipts     SpanName = "ver.finder.checkReadyReceipts"
	VERFindProcessResult          SpanName = "ver.finder.processResult"
	VERFindOnResultProcessed      SpanName = "ver.finder.onResultProcessed"

	VERProcessExecutionResult SpanName = "ver.processExecutionResult"
	// children of VERProcessExecutionResult
	VERMatchHandleExecutionResult SpanName = "ver.match.handleExecutionResult"
	VERMatchHandleChunkDataPack   SpanName = "ver.match.handleChunkDataPack"
	VERMatchMyChunkAssignments    SpanName = "ver.match.myChunkAssignments"
	VERVerVerifyWithMetrics       SpanName = "ver.verify.verifyWithMetrics"
	VERVerChunkVerify             SpanName = "ver.verify.ChunkVerifier.Verify"
	VERVerGenerateResultApproval  SpanName = "ver.verify.GenerateResultApproval"
)

// Tag names
const (
	EXEParseDurationTag     = "runtime.parseTransactionDuration"
	EXECheckDurationTag     = "runtime.checkTransactionDuration"
	EXEInterpretDurationTag = "runtime.interpretTransactionDuration"
)
