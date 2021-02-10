package trace

// Span names
const (

	// Common
	//

	// State
	// mutator.Extend - full payload check
	ProtoStateMutatorExtend                SpanName = "common.state.proto.mutator.extend"
	ProtoStateMutatorExtendCheckHeader              = "common.state.proto.mutator.extend.checkHeader"
	ProtoStateMutatorExtendCheckGuarantees          = "common.state.proto.mutator.extend.checkGuarantees"
	ProtoStateMutatorExtendCheckSeals               = "common.state.proto.mutator.extend.checkSeals"
	ProtoStateMutatorExtendCheckReceipts            = "common.state.proto.mutator.extend.checkReceipts"
	ProtoStateMutatorExtendDBInsert                 = "common.state.proto.mutator.extend.dbInsert"

	// mutator.HeaderExtend - header-only check
	ProtoStateMutatorHeaderExtend              = "common.state.proto.mutator.headerExtend"
	ProtoStateMutatorHeaderExtendGetLastSealed = "common.state.proto.mutator.headerExtend.lastSealed"

	// mutator.Finalize
	ProtoStateMutatorFinalize = "common.state.proto.mutator.finalize"

	// Consensus Node
	//

	CONProcessCollection = "con.processCollection"
	CONProcessBlock      = "con.processBlock"

	// Hotstuff
	CONHotFinalizeCollection = "con.hotstuff.finalizeCollection"
	CONHotFinalizeBlock      = "con.hotstuff.finalizeBlock"

	// Ingestion
	CONIngOnCollectionGuarantee = "con.ingestion.onCollectionGuarantee"

	// Propagation
	CONPropOnGuarantee = "con.propagation.onGuarantee"

	// Provider
	CONProvOnBlockProposal = "con.provider.onBlockProposal"

	// Compliance
	CONCompBroadcastProposalWithDelay      = "con.compliance.BroadcastProposalWithDelay"
	CONCompOnBlockProposal                 = "con.compliance.onBlockProposal"
	CONCompOnBlockProposalProcessRecursive = "con.compliance.onBlockProposal.processBlockProposal.recursive"
	CONCompOnBlockProposalProcessSingle    = "con.compliance.onBlockProposal.processBlockProposal.single"

	// Matching
	CONMatchCheckSealing                        = "con.matching.checkSealing"
	CONMatchCheckSealingSealableResults         = "con.matching.checkSealing.sealableResults"
	CONMatchCheckSealingClearPools              = "con.matching.checkSealing.clearPools"
	CONMatchCheckSealingRequestPendingReceipts  = "con.matching.checkSealing.requestPendingReceipts"
	CONMatchCheckSealingRequestPendingApprovals = "con.matching.checkSealing.requestPendingApprovals"
	CONMatchOnReceipt                           = "con.matching.onReceipt"
	CONMatchOnReceiptVal                        = "con.matching.onReceipt.validation"
	CONMatchOnApproval                          = "con.matching.onApproval"

	// Builder
	CONBuildOn                        = "con.builder"
	CONBuildOnCreatePayloadGuarantees = "con.builder.createPayload.guarantees"
	CONBuildOnCreatePayloadSeals      = "con.builder.createPayload.seals"
	CONBuildOnCreatePayloadReceipts   = "con.builder.createPayload.receipts"
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

	EXEExecuteBlock           = "exe.ingestion.executeBlock"
	EXESaveExecutionResults   = "exe.ingestion.saveExecutionResults"
	EXESaveExecutionReceipt   = "exe.ingestion.saveExecutionReceipt"
	EXESaveTransactionResults = "exe.ingestion.saveTransactionResults"
	EXESaveTransactionEvents  = "exe.ingestion.saveTransactionEvents"

	EXEBroadcastExecutionReceipt = "exe.provider.broadcastExecutionReceipt"

	EXEComputeBlock            = "exe.computer.computeBlock"
	EXEComputeCollection       = "exe.computer.computeCollection"
	EXEComputeSystemCollection = "exe.computer.computeSystemCollection"
	EXEComputeTransaction      = "exe.computer.computeTransaction"

	EXECommitDelta                        = "exe.state.commitDelta"
	EXEGetRegisters                       = "exe.state.getRegisters"
	EXEGetRegistersWithProofs             = "exe.state.getRegistersWithProofs"
	EXEPersistStateCommitment             = "exe.state.persistStateCommitment"
	EXEPersistChunkDataPack               = "exe.state.persistChunkDataPack"
	EXEGetExecutionResultID               = "exe.state.getExecutionResultID"
	EXEPersistExecutionResult             = "exe.state.persistExecutionResult"
	EXEPersistStateInteractions           = "exe.state.persistStateInteractions"
	EXERetrieveStateDelta                 = "exe.state.retrieveStateDelta"
	EXEUpdateHighestExecutedBlockIfHigher = "exe.state.updateHighestExecutedBlockIfHigher"
	EXEGetHighestExecutedBlockID          = "exe.state.getHighestExecutedBlockID"

	// Verification node
	//

	VERProcessExecutionReceipt = "ver.processExecutionReceipt"
	// children of VERProcessExecutionReceipt
	VERFindHandleExecutionReceipt = "ver.find.handleExecutionReceipt"
	VERFindOnFinalizedBlock       = "ver.finder.OnFinalizedBlock"
	VERFindCheckCachedReceipts    = "ver.finder.checkCachedReceipts"
	VERFindCheckPendingReceipts   = "ver.finder.checkPendingReceipts"
	VERFindCheckReadyReceipts     = "ver.finder.checkReadyReceipts"
	VERFindProcessResult          = "ver.finder.processResult"
	VERFindOnResultProcessed      = "ver.finder.onResultProcessed"

	VERProcessExecutionResult = "ver.processExecutionResult"
	// children of VERProcessExecutionResult
	VERMatchHandleExecutionResult = "ver.match.handleExecutionResult"
	VERMatchHandleChunkDataPack   = "ver.match.handleChunkDataPack"
	VERMatchMyChunkAssignments    = "ver.match.myChunkAssignments"
	VERVerVerifyWithMetrics       = "ver.verify.verifyWithMetrics"
	VERVerChunkVerify             = "ver.verify.ChunkVerifier.Verify"
	VERVerGenerateResultApproval  = "ver.verify.GenerateResultApproval"
)

// Tag names
const (
	EXEParseDurationTag         = "runtime.parseTransactionDuration"
	EXECheckDurationTag         = "runtime.checkTransactionDuration"
	EXEInterpretDurationTag     = "runtime.interpretTransactionDuration"
	EXEValueEncodingDurationTag = "runtime.encodingValueDuration"
	EXEValueDecodingDurationTag = "runtime.decodingValueDuration"
)
