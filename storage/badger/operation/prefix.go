package operation

import (
	op "github.com/onflow/flow-go/storage/operation"
)

const (

	// codes for special database markers
	codeMax    = 1 // keeps track of the maximum key size
	codeDBType = 2 // specifies a database type

	// codes for views with special meaning
	codeSafetyData   = 10 // safety data for hotstuff state
	codeLivenessData = 11 // liveness data for hotstuff state

	// codes for fields associated with the root state
	codeSporkID              = 13
	_                        = 14 // DEPRECATED: 14 was used for ProtocolVersion before the versioned Protocol State
	_                        = 15 // DEPRECATED: 15 was used to save the finalization safety threshold
	codeSporkRootBlockHeight = 16

	// code for heights with special meaning
	codeFinalizedHeight         = 20 // latest finalized block height
	codeSealedHeight            = 21 // latest sealed block height
	codeClusterHeight           = 22 // latest finalized height on cluster
	codeExecutedBlock           = 23 // latest executed block with max height
	codeFinalizedRootHeight     = 24 // the height of the highest finalized block contained in the root snapshot
	codeLastCompleteBlockHeight = 25 // the height of the last block for which all collections were received
	codeEpochFirstHeight        = 26 // the height of the first block in a given epoch
	codeSealedRootHeight        = 27 // the height of the highest sealed block contained in the root snapshot

	// codes for single entity storage
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
	codeHeightToBlock               = 40 // index mapping height to block ID
	codeBlockIDToLatestSealID       = 41 // index mapping a block its last payload seal
	codeClusterBlockToRefBlock      = 42 // index cluster block ID to reference block ID
	codeRefHeightToClusterBlock     = 43 // index reference block height to cluster block IDs
	codeBlockIDToFinalizedSeal      = 44 // index _finalized_ seal by sealed block ID
	codeBlockIDToQuorumCertificate  = 45 // index of quorum certificates by block ID
	codeEpochProtocolStateByBlockID = 46 // index of epoch protocol state entry by block ID
	codeProtocolKVStoreByBlockID    = 47 // index of protocol KV store entry by block ID

	// codes for indexing multiple identifiers by identifier
	codeBlockChildren          = 50 // index mapping block ID to children blocks
	_                          = 51 // DEPRECATED: 51 was used for identity indexes before epochs
	codePayloadGuarantees      = 52 // index mapping block ID to payload guarantees
	codePayloadSeals           = 53 // index mapping block ID to payload seals
	codeCollectionBlock        = 54 // index mapping collection ID to block ID
	codeOwnBlockReceipt        = 55 // index mapping block ID to execution receipt ID for execution nodes
	_                          = 56 // DEPRECATED: 56 was used for block->epoch status prior to Dynamic Protocol State in Mainnet25
	codePayloadReceipts        = 57 // index mapping block ID to payload receipts
	codePayloadResults         = 58 // index mapping block ID to payload results
	codeAllBlockReceipts       = 59 // index mapping of blockID to multiple receipts
	codePayloadProtocolStateID = 60 // index mapping block ID to payload protocol state ID

	// codes related to protocol level information
	codeEpochSetup         = 61 // EpochSetup service event, keyed by ID
	codeEpochCommit        = 62 // EpochCommit service event, keyed by ID
	codeBeaconPrivateKey   = 63 // BeaconPrivateKey, keyed by epoch counter
	_                      = 64 // [DEPRECATED] flag that the DKG for an epoch has been started, used in protocol version v1
	codeDKGEndState        = 65 // [DEPRECATED] flag for DKG end state, used in protocol version v1
	codeDKGState           = 66 // current state of Recoverable Random Beacon State Machine for given epoch
	codeVersionBeacon      = 67 // flag for storing version beacons
	codeEpochProtocolState = 68
	codeProtocolKVStore    = 69

	// code for ComputationResult upload status storage
	// NOTE: for now only GCP uploader is supported. When other uploader (AWS e.g.) needs to
	//		 be supported, we will need to define new code.
	_ = 66 // used by ComputationResults in storage/operation

	// job queue consumers and producers
	codeJobConsumerProcessed = 70
	codeJobQueue             = 71
	codeJobQueuePointer      = 72

	// legacy codes (should be cleaned up)
	codeChunkDataPack                      = 100
	codeCommit                             = 101
	codeEvent                              = 102
	codeExecutionStateInteractions         = 103
	codeTransactionResult                  = 104
	codeFinalizedCluster                   = 105
	codeServiceEvent                       = 106
	codeTransactionResultIndex             = 107
	codeLightTransactionResult             = 108
	codeLightTransactionResultIndex        = 109
	codeTransactionResultErrorMessage      = 110
	codeTransactionResultErrorMessageIndex = 111
	codeIndexCollection                    = 200
	codeIndexExecutionResultByBlock        = 202
	codeIndexCollectionByTransaction       = 203
	codeIndexResultApprovalByChunk         = 204

	// TEMPORARY codes
	disallowedNodeIDs = 205 // manual override for adding node IDs to list of ejected nodes, applies to networking layer only

	// internal failure information that should be preserved across restarts
	codeExecutionFork                   = 254
	codeEpochEmergencyFallbackTriggered = 255
)

func makePrefix(code byte, keys ...any) []byte {
	return op.MakePrefix(code, keys...)
}

func keyPartToBinary(v any) []byte {
	return op.AppendPrefixKeyPart(nil, v)
}
