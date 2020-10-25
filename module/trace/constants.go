package trace

// Span names
const (

	// Common
	//

	// State
	// mutator.Extend - full payload check
	ProtoStateMutatorExtend                = "common.state.proto.mutator.extend"
	ProtoStateMutatorExtendCheckHeader     = "common.state.proto.mutator.extend.checkHeader"
	ProtoStateMutatorExtendCheckGuarantees = "common.state.proto.mutator.extend.checkGuarantees"
	ProtoStateMutatorExtendCheckSeals      = "common.state.proto.mutator.extend.checkSeals"
	ProtoStateMutatorExtendDBInsert        = "common.state.proto.mutator.extend.dbInsert"

	// mutator.HeaderExtend - header-only check
	ProtoStateMutatorHeaderExtend              = "common.state.proto.mutator.headerExtend"
	ProtoStateMutatorHeaderExtendGetLastSealed = "common.state.proto.mutator.headerExtend.lastSealed"

	// mutator.Finalize
	ProtoStateMutatorFinalize = "common.state.proto.mutator.finalize"

	// Consensus Node
	//

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
	CONHotFinalizeBlock SpanName = "con.hotstuff.finalizeBlock"

	// Builder
	CONBuildOn                        = "con.builder"
	CONBuildOnSetup                   = "con.builder.setup"
	CONBuildOnUnfinalizedLookup       = "con.builder.unfinalizedLookup"
	CONBuildOnFinalizedLookup         = "con.builder.finalizedLookup"
	CONBuildOnCreatePayloadGuarantees = "con.builder.createPayload.guarantees"
	CONBuildOnCreatePayloadSeals      = "con.builder.createPayload.seals"
	CONBuildOnCreateHeader            = "con.builder.createHeader"
	CONBuildOnDBInsert                = "con.builder.dbInsert"

	// Collection Node
	//

	// Builder
	COLBuildOn                  = "col.builder"
	COLBuildOnSetup             = "col.builder.setup"
	COLBuildOnUnfinalizedLookup = "col.builder.unfinalizedLookup"
	COLBuildOnFinalizedLookup   = "col.builder.finalizedLookup"
	COLBuildOnCreatePayload     = "col.builder.createPayload"
	COLBuildOnCreateHeader      = "col.builder.createHeader"
	COLBuildOnDBInsert          = "col.builder.dbInsert"

	// Cluster State
	COLClusterStateMutatorExtend                       = "col.state.mutator.extend"
	COLClusterStateMutatorExtendSetup                  = "col.state.mutator.extend.setup"
	COLClusterStateMutatorExtendCheckAncestry          = "col.state.mutator.extend.ancestry"
	COLClusterStateMutatorExtendCheckTransactionsValid = "col.state.mutator.extend.transactions.validity"
	COLClusterStateMutatorExtendCheckTransactionsDupes = "col.state.mutator.extend.transactions.dupes"
	COLClusterStateMutatorExtendDBInsert               = "col.state.mutator.extend.dbInsert"

	// Execution Node
	//

	EXEExecuteBlock           SpanName = "exe.ingestion.executeBlock"
	EXESaveExecutionResults   SpanName = "exe.ingestion.saveExecutionResults"
	EXESaveExecutionReceipt   SpanName = "exe.ingestion.saveExecutionReceipt"
	EXESaveTransactionResults SpanName = "exe.ingestion.saveTransactionResults"
	EXESaveTransactionEvents  SpanName = "exe.ingestion.saveTransactionEvents"

	EXEBroadcastExecutionReceipt SpanName = "exe.provider.broadcastExecutionReceipt"

	EXEComputeBlock            SpanName = "exe.computer.computeBlock"
	EXEComputeCollection       SpanName = "exe.computer.computeCollection"
	EXEComputeSystemCollection SpanName = "exe.computer.computeSystemCollection"
	EXEComputeTransaction      SpanName = "exe.computer.computeTransaction"

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
	//

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
