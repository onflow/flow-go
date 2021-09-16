package trace

// Span names
const (

	// Protocol State
	//

	// Extend
	ProtoStateMutatorExtend                SpanName = "proto.state.mutator.extend"
	ProtoStateMutatorExtendCheckHeader     SpanName = "proto.state.mutator.extend.checkHeader"
	ProtoStateMutatorExtendCheckGuarantees SpanName = "proto.state.mutator.extend.checkGuarantees"
	ProtoStateMutatorExtendCheckSeals      SpanName = "proto.state.mutator.extend.checkSeals"
	ProtoStateMutatorExtendCheckReceipts   SpanName = "proto.state.mutator.extend.checkReceipts"
	ProtoStateMutatorExtendDBInsert        SpanName = "proto.state.mutator.extend.dbInsert"

	// HeaderExtend
	ProtoStateMutatorHeaderExtend              SpanName = "proto.state.mutator.headerExtend"
	ProtoStateMutatorHeaderExtendGetLastSealed SpanName = "proto.state.mutator.headerExtend.lastSealed"

	// mutator.Finalize
	ProtoStateMutatorFinalize SpanName = "proto.state.mutator.finalize"

	// Consensus
	//

	// Builder
	CONBuilderBuildOn SpanName = "con.builder.buildOn"

	// Finalizer
	CONFinalizerFinalizeBlock SpanName = "con.finalizer.finalizeBlock"

	// Ingestion
	CONIngOnCollectionGuarantee SpanName = "con.ingestion.onCollectionGuarantee"

	// Provider

	// Compliance
	CONCompOnBlockProposal      SpanName = "con.compliance.onBlockProposal"
	ConCompProcessBlockProposal SpanName = "con.compliance.processBlockProposal"

	// Matching
	CONMatchProcessReceipt    SpanName = "con.matching.processReceipt"
	CONMatchProcessReceiptVal SpanName = "con.matching.processReceipt.validation"

	// Sealing
	CONSealingProcessFinalizedBlock           SpanName = "con.sealing.processFinalizedBlock"
	CONSealingCheckForEmergencySealableBlocks SpanName = "con.sealing.processFinalizedBlock.checkEmergencySealing"
	CONSealingPruning                         SpanName = "con.sealing.processFinalizedBlock.pruning"
	CONSealingRequestingPendingApproval       SpanName = "con.sealing.processFinalizedBlock.requestPendingApprovals"
	CONSealingProcessIncorporatedResult       SpanName = "con.sealing.processIncorporatedResult"
	CONSealingProcessApproval                 SpanName = "con.sealing.processApproval"

	//Follower Engine
	FollowerOnBlockProposal        SpanName = "follower.onBlockProposal"
	FollowerProcessBlockProposal   SpanName = "follower.processBlockProposal"
	FollowerProcessPendingChildren SpanName = "follower.processPendingChildren"

	// Collection Node
	//

	// Builder
	COLBuildOn                  SpanName = "col.builder"
	COLBuildOnSetup             SpanName = "col.builder.setup"
	COLBuildOnUnfinalizedLookup SpanName = "col.builder.unfinalizedLookup"
	COLBuildOnFinalizedLookup   SpanName = "col.builder.finalizedLookup"
	COLBuildOnCreatePayload     SpanName = "col.builder.createPayload"
	COLBuildOnCreateHeader      SpanName = "col.builder.createHeader"
	COLBuildOnDBInsert          SpanName = "col.builder.dbInsert"

	// Cluster State
	COLClusterStateMutatorExtend                       SpanName = "col.state.mutator.extend"
	COLClusterStateMutatorExtendSetup                  SpanName = "col.state.mutator.extend.setup"
	COLClusterStateMutatorExtendCheckAncestry          SpanName = "col.state.mutator.extend.ancestry"
	COLClusterStateMutatorExtendCheckTransactionsValid SpanName = "col.state.mutator.extend.transactions.validity"
	COLClusterStateMutatorExtendCheckTransactionsDupes SpanName = "col.state.mutator.extend.transactions.dupes"
	COLClusterStateMutatorExtendDBInsert               SpanName = "col.state.mutator.extend.dbInsert"

	// Execution Node
	//

	EXEHandleBlock             SpanName = "exe.ingestion.handleBlock"
	EXEHandleCollection        SpanName = "exe.ingestion.handleCollection"
	EXEHandleComputationResult SpanName = "exe.ingestion.handleComputationResult"
	EXEExecuteBlock            SpanName = "exe.ingestion.executeBlock"
	EXESaveExecutionResults    SpanName = "exe.ingestion.saveExecutionResults"

	EXEBroadcastExecutionReceipt SpanName = "exe.provider.broadcastExecutionReceipt"

	EXEComputeBlock            SpanName = "exe.computer.computeBlock"
	EXEComputeCollection       SpanName = "exe.computer.computeCollection"
	EXEComputeSystemCollection SpanName = "exe.computer.computeSystemCollection"
	EXEComputeTransaction      SpanName = "exe.computer.computeTransaction"
	EXERunTransaction          SpanName = "exe.computer.runTransaction"
	EXEMergeTransactionView    SpanName = "exe.computer.mergeTransactionView"

	EXEStatePersistExecutionState         SpanName = "exe.state.persistExecutionState"
	EXECommitDelta                        SpanName = "exe.state.commitDelta"
	EXEGetRegisters                       SpanName = "exe.state.getRegisters"
	EXEGetRegistersWithProofs             SpanName = "exe.state.getRegistersWithProofs"
	EXEGetExecutionResultID               SpanName = "exe.state.getExecutionResultID"
	EXEUpdateHighestExecutedBlockIfHigher SpanName = "exe.state.updateHighestExecutedBlockIfHigher"
	EXEHashEvents                         SpanName = "exe.state.hashEvents"

	// Verification node
	//
	// assigner engine
	VERProcessFinalizedBlock SpanName = "ver.processFinalizedBlock"
	// children of VERProcessFinalizedBlock
	VERAssignerHandleFinalizedBlock   SpanName = "ver.assigner.handleFinalizedBlock"
	VERAssignerHandleExecutionReceipt SpanName = "ver.assigner.handleExecutionReceipt"
	VERAssignerChunkAssignment        SpanName = "ver.assigner.chunkAssignment"
	VERAssignerProcessChunk           SpanName = "ver.assigner.processChunk"

	// fetcher engine
	VERProcessAssignedChunk SpanName = "ver.processAssignedChunk"
	// children of VERProcessAssignedChunk
	VERFetcherHandleAssignedChunk   SpanName = "ver.fetcher.handleAssignedChunk"
	VERFetcherHandleChunkDataPack   SpanName = "ver.fetcher.handleChunkDataPack"
	VERFetcherValidateChunkDataPack SpanName = "ver.fetcher.validateChunkDataPack"
	VERFetcherPushToVerifier        SpanName = "ver.fetcher.pushToVerifier"

	// requester engine
	VERProcessChunkDataPackRequest SpanName = "ver.processChunkDataPackRequest"
	// children of VERProcessChunkDataPackRequest
	VERRequesterHandleChunkDataRequest   SpanName = "ver.requester.handleChunkDataRequest"
	VERRequesterHandleChunkDataResponse  SpanName = "ver.requester.handleChunkDataResponse"
	VERRequesterDispatchChunkDataRequest SpanName = "ver.requester.dispatchChunkDataRequest"

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

	// Flow Virtual Machine
	FVMVerifyTransaction             SpanName = "fvm.verifyTransaction"
	FVMSeqNumCheckTransaction        SpanName = "fvm.seqNumCheckTransaction"
	FVMExecuteTransaction            SpanName = "fvm.executeTransaction"
	FVMDeductTransactionFees         SpanName = "fvm.deductTransactionFees"
	FVMInvokeContractFunction        SpanName = "fvm.invokeContractFunction"
	FVMFrozenAccountCheckTransaction SpanName = "fvm.frozenAccountCheckTransaction"

	FVMEnvHash                      SpanName = "fvm.env.Hash"
	FVMEnvValueExists               SpanName = "fvm.env.valueExists"
	FVMEnvGetValue                  SpanName = "fvm.env.getValue"
	FVMEnvSetValue                  SpanName = "fvm.env.setValue"
	FVMEnvGetStorageUsed            SpanName = "fvm.env.getStorageUsed"
	FVMEnvGetStorageCapacity        SpanName = "fvm.env.getStorageCapacity"
	FVMEnvGetAccountBalance         SpanName = "fvm.env.getAccountBalance"
	FVMEnvResolveLocation           SpanName = "fvm.env.resolveLocation"
	FVMEnvGetCode                   SpanName = "fvm.env.getCode"
	FVMEnvGetAccountContractNames   SpanName = "fvm.env.getAccountContractNames"
	FVMEnvGetProgram                SpanName = "fvm.env.getCachedProgram"
	FVMEnvSetProgram                SpanName = "fvm.env.cacheProgram"
	FVMEnvProgramLog                SpanName = "fvm.env.programLog"
	FVMEnvEmitEvent                 SpanName = "fvm.env.emitEvent"
	FVMEnvGenerateUUID              SpanName = "fvm.env.generateUUID"
	FVMEnvDecodeArgument            SpanName = "fvm.env.decodeArgument"
	FVMEnvVerifySignature           SpanName = "fvm.env.verifySignature"
	FVMEnvGetCurrentBlockHeight     SpanName = "fvm.env.getCurrentBlockHeight"
	FVMEnvUnsafeRandom              SpanName = "fvm.env.unsafeRandom"
	FVMEnvGetBlockAtHeight          SpanName = "fvm.env.getBlockAtHeight"
	FVMEnvCreateAccount             SpanName = "fvm.env.createAccount"
	FVMEnvAddAccountKey             SpanName = "fvm.env.addAccountKey"
	FVMEnvGetAccountKey             SpanName = "fvm.env.getAccountKey"
	FVMEnvRemoveAccountKey          SpanName = "fvm.env.removeAccountKey"
	FVMEnvUpdateAccountContractCode SpanName = "fvm.env.updateAccountContractCode"
	FVMEnvGetAccountContractCode    SpanName = "fvm.env.getAccountContractCode"
	FVMEnvRemoveAccountContractCode SpanName = "fvm.env.removeAccountContractCode"
	FVMEnvGetSigningAccounts        SpanName = "fvm.env.getSigningAccounts"

	FVMCadenceParseProgram     SpanName = "fvm.cadence.parseProgram"
	FVMCadenceCheckProgram     SpanName = "fvm.cadence.checkProgram"
	FVMCadenceInterpretProgram SpanName = "fvm.cadence.interpretProgram"
	FVMCadenceEncodeValue      SpanName = "fvm.cadence.encodeValue"
	FVMCadenceDecodeValue      SpanName = "fvm.cadence.decodeValue"
)

// Tag names
const (
	EXEParseDurationTag         = "runtime.parseTransactionDuration"
	EXECheckDurationTag         = "runtime.checkTransactionDuration"
	EXEInterpretDurationTag     = "runtime.interpretTransactionDuration"
	EXEValueEncodingDurationTag = "runtime.encodingValueDuration"
	EXEValueDecodingDurationTag = "runtime.decodingValueDuration"
)
