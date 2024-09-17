package operation

const (

	// codes for special database markers
	// codeMax    = 1 // keeps track of the maximum key size
	// nolint:unused
	codeDBType = 2 // specifies a database type

	// codes for views with special meaning
	// nolint:unused
	codeSafetyData = 10 // safety data for hotstuff state
	// nolint:unused
	codeLivenessData = 11 // liveness data for hotstuff state

	// codes for fields associated with the root state
	// nolint:unused
	codeSporkID = 13
	// nolint:unused
	codeProtocolVersion = 14
	// nolint:unused
	codeEpochCommitSafetyThreshold = 15
	// nolint:unused
	codeSporkRootBlockHeight = 16

	// code for heights with special meaning
	// nolint:unused
	codeFinalizedHeight = 20 // latest finalized block height
	// nolint:unused
	codeSealedHeight = 21 // latest sealed block height
	// nolint:unused
	codeClusterHeight = 22 // latest finalized height on cluster
	// nolint:unused
	codeExecutedBlock = 23 // latest executed block with max height
	// nolint:unused
	codeFinalizedRootHeight = 24 // the height of the highest finalized block contained in the root snapshot
	// nolint:unused
	codeLastCompleteBlockHeight = 25 // the height of the last block for which all collections were received
	// nolint:unused
	codeEpochFirstHeight = 26 // the height of the first block in a given epoch
	// nolint:unused
	codeSealedRootHeight = 27 // the height of the highest sealed block contained in the root snapshot

	// codes for single entity storage
	// nolint:unused
	codeHeader               = 30
	_                        = 31 // DEPRECATED: 31 was used for identities before epochs
	codeGuarantee            = 32
	codeSeal                 = 33
	codeTransaction          = 34
	codeCollection           = 35
	codeExecutionResult      = 36
	codeResultApproval       = 37
	codeChunk                = 38
	codeExecutionReceiptMeta = 39 // NOTE: prior to Mainnet25, this erroneously had the same value as codeExecutionResult (36)

	// codes for indexing single identifier by identifier/integer
	// nolint:unused
	codeHeightToBlock = 40 // index mapping height to block ID
	// nolint:unused
	codeBlockIDToLatestSealID = 41 // index mapping a block its last payload seal
	// nolint:unused
	codeClusterBlockToRefBlock = 42 // index cluster block ID to reference block ID
	// nolint:unused
	codeRefHeightToClusterBlock = 43 // index reference block height to cluster block IDs
	// nolint:unused
	codeBlockIDToFinalizedSeal = 44 // index _finalized_ seal by sealed block ID
	// nolint:unused
	codeBlockIDToQuorumCertificate = 45 // index of quorum certificates by block ID
	// nolint:unused
	codeEpochProtocolStateByBlockID = 46 // index of epoch protocol state entry by block ID
	// nolint:unused
	codeProtocolKVStoreByBlockID = 47 // index of protocol KV store entry by block ID

	// codes for indexing multiple identifiers by identifier
	// nolint:unused
	codeBlockChildren = 50 // index mapping block ID to children blocks
	_                 = 51 // DEPRECATED: 51 was used for identity indexes before epochs
	// nolint:unused
	codePayloadGuarantees = 52 // index mapping block ID to payload guarantees
	// nolint:unused
	codePayloadSeals = 53 // index mapping block ID to payload seals
	// nolint:unused
	codeCollectionBlock = 54 // index mapping collection ID to block ID
	// nolint:unused
	codeOwnBlockReceipt = 55 // index mapping block ID to execution receipt ID for execution nodes
	_                   = 56 // DEPRECATED: 56 was used for block->epoch status prior to Dynamic Protocol State in Mainnet25
	// nolint:unused
	codePayloadReceipts = 57 // index mapping block ID to payload receipts
	// nolint:unused
	codePayloadResults = 58 // index mapping block ID to payload results
	// nolint:unused
	codeAllBlockReceipts = 59 // index mapping of blockID to multiple receipts
	// nolint:unused
	codePayloadProtocolStateID = 60 // index mapping block ID to payload protocol state ID

	// codes related to protocol level information
	// nolint:unused
	codeEpochSetup = 61 // EpochSetup service event, keyed by ID
	// nolint:unused
	codeEpochCommit = 62 // EpochCommit service event, keyed by ID
	// nolint:unused
	codeBeaconPrivateKey = 63 // BeaconPrivateKey, keyed by epoch counter
	// nolint:unused
	codeDKGStarted = 64 // flag that the DKG for an epoch has been started
	// nolint:unused
	codeDKGEnded = 65 // flag that the DKG for an epoch has ended (stores end state)
	// nolint:unused
	codeVersionBeacon = 67 // flag for storing version beacons
	// nolint:unused
	codeEpochProtocolState = 68
	// nolint:unused
	codeProtocolKVStore = 69

	// code for ComputationResult upload status storage
	// NOTE: for now only GCP uploader is supported. When other uploader (AWS e.g.) needs to
	//		 be supported, we will need to define new code.
	// nolint:unused
	codeComputationResults = 66

	// job queue consumers and producers
	// nolint:unused
	codeJobConsumerProcessed = 70
	// nolint:unused
	codeJobQueue = 71
	// nolint:unused
	codeJobQueuePointer = 72

	// legacy codes (should be cleaned up)
	codeChunkDataPack                = 100
	codeCommit                       = 101
	codeEvent                        = 102
	codeExecutionStateInteractions   = 103
	codeTransactionResult            = 104
	codeFinalizedCluster             = 105
	codeServiceEvent                 = 106
	codeTransactionResultIndex       = 107
	codeLightTransactionResult       = 108
	codeLightTransactionResultIndex  = 109
	codeIndexCollection              = 200
	codeIndexExecutionResultByBlock  = 202
	codeIndexCollectionByTransaction = 203
	codeIndexResultApprovalByChunk   = 204

	// TEMPORARY codes
	// nolint:unused
	blockedNodeIDs = 205 // manual override for adding node IDs to list of ejected nodes, applies to networking layer only

	// internal failure information that should be preserved across restarts
	// nolint:unused
	codeExecutionFork = 254
	// nolint:unused
	codeEpochEmergencyFallbackTriggered = 255
)

func makePrefix(code byte, keys ...interface{}) []byte {
	prefix := make([]byte, 1)
	prefix[0] = code
	for _, key := range keys {
		prefix = append(prefix, EncodeKeyPart(key)...)
	}
	return prefix
}
