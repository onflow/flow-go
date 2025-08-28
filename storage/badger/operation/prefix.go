package operation

import (
	op "github.com/onflow/flow-go/storage/operation"
)

const (

	// codes for special database markers
	codeMax    = 1 // keeps track of the maximum key size
	codeDBType = 2 // specifies a database type

<<<<<<< HEAD
=======
	// codes for views with special meaning
	codeSafetyData   = 10 // safety data for hotstuff state
	codeLivenessData = 11 // liveness data for hotstuff state

	// codes for fields associated with the root state
	_                    = 13 // DEPRECATED: 13 was used for root spork ID
	_                    = 14 // DEPRECATED: 14 was used for ProtocolVersion before the versioned Protocol State
	_                    = 15 // DEPRECATED: 15 was used to save the finalization safety threshold
	_                    = 16 // DEPRECATED: 16 was used for root spork height
	codeSporkRootBlockID = 17 // the root spork block ID

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
	codeExecutionReceiptStub = 39 // NOTE: prior to Mainnet25, this erroneously had the same value as codeExecutionResult (36)

	// codes for indexing single identifier by identifier/integer
	codeHeightToBlock               = 40 // index mapping height to block ID
	codeBlockIDToLatestSealID       = 41 // index mapping a block its last payload seal
	codeClusterBlockToRefBlock      = 42 // index cluster block ID to reference block ID
	codeRefHeightToClusterBlock     = 43 // index reference block height to cluster block IDs
	codeBlockIDToFinalizedSeal      = 44 // index _finalized_ seal by sealed block ID
	codeBlockIDToQuorumCertificate  = 45 // index of quorum certificates by block ID
	codeEpochProtocolStateByBlockID = 46 // index of epoch protocol state entry by block ID
	codeProtocolKVStoreByBlockID    = 47 // index of protocol KV store entry by block ID
	codeBlockIDToProposalSignature  = 48 // index of proposer signatures by block ID
	codeGuaranteeByCollectionID     = 49 // index of collection guarantee by collection ID

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

>>>>>>> master
	// codes related to protocol level information
	codeBeaconPrivateKey = 63 // BeaconPrivateKey, keyed by epoch counter
	_                    = 64 // DEPRECATED: flag that the DKG for an epoch has been started
	_                    = 65 // DEPRECATED: flag that the DKG for an epoch has ended (stores end state)
	codeDKGState         = 66 // current state of Recoverable Random Beacon State Machine for given epoch
)

func makePrefix(code byte, keys ...any) []byte {
	return op.MakePrefix(code, keys...)
}

func keyPartToBinary(v any) []byte {
	return op.AppendPrefixKeyPart(nil, v)
}
