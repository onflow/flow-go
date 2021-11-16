// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package operation

import (
	"encoding/binary"
	"fmt"

	"github.com/onflow/flow-go/model/flow"
)

const (

	// codes for special database markers
	codeMax    = 1 // keeps track of the maximum key size
	codeDBType = 2 // specifies a database type

	// codes for views with special meaning
	codeStartedView = 10 // latest view hotstuff started
	codeVotedView   = 11 // latest view hotstuff voted on

	// codes for fields associated with the root state
	codeRootQuorumCertificate = 12
	codeSporkID               = 13
	codeProtocolVersion       = 14

	// code for heights with special meaning
	codeFinalizedHeight         = 20 // latest finalized block height
	codeSealedHeight            = 21 // latest sealed block height
	codeClusterHeight           = 22 // latest finalized height on cluster
	codeExecutedBlock           = 23 // latest executed block with max height
	codeRootHeight              = 24 // the height of the first loaded block
	codeLastCompleteBlockHeight = 25 // the height of the last block for which all collections were received

	// codes for single entity storage
	// 31 was used for identities before epochs
	codeHeader               = 30
	codeGuarantee            = 32
	codeSeal                 = 33
	codeTransaction          = 34
	codeCollection           = 35
	codeExecutionResult      = 36
	codeExecutionReceiptMeta = 36
	codeResultApproval       = 37
	codeChunk                = 38

	// codes for indexing single identifier by identifier
	codeHeightToBlock       = 40 // index mapping height to block ID
	codeBlockToSeal         = 41 // index mapping a block its last payload seal
	codeCollectionReference = 42 // index reference block ID for collection
	codeBlockValidity       = 43 // validity of block per HotStuff

	// codes for indexing multiple identifiers by identifier
	// NOTE: 51 was used for identity indexes before epochs
	codeBlockChildren       = 50 // index mapping block ID to children blocks
	codePayloadGuarantees   = 52 // index mapping block ID to payload guarantees
	codePayloadSeals        = 53 // index mapping block ID to payload seals
	codeCollectionBlock     = 54 // index mapping collection ID to block ID
	codeOwnBlockReceipt     = 55 // index mapping block ID to execution receipt ID for execution nodes
	codeBlockEpochStatus    = 56 // index mapping block ID to epoch status
	codePayloadReceipts     = 57 // index mapping block ID  to payload receipts
	codePayloadResults      = 58 // index mapping block ID to payload results
	codeAllBlockReceipts    = 59 // index mapping of blockID to multiple receipts
	codeIndexBlockByChunkID = 60 // index mapping chunk ID to block ID

	// codes related to epoch information
	codeEpochSetup     = 61 // EpochSetup service event, keyed by ID
	codeEpochCommit    = 62 // EpochCommit service event, keyed by ID
	codeDKGPrivateInfo = 63 // DKGPrivateInfo, keyed by epoch counter
	codeDKGStarted     = 64 // flag that the DKG for an epoch has been started

	// job queue consumers and producers
	codeJobConsumerProcessed = 70
	codeJobQueue             = 71
	codeJobQueuePointer      = 72

	// legacy codes (should be cleaned up)
	codeChunkDataPack                = 100
	codeCommit                       = 101
	codeEvent                        = 102
	codeExecutionStateInteractions   = 103
	codeTransactionResult            = 104
	codeFinalizedCluster             = 105
	codeServiceEvent                 = 106
	codeIndexCollection              = 200
	codeIndexExecutionResultByBlock  = 202
	codeIndexCollectionByTransaction = 203
	codeIndexResultApprovalByChunk   = 204

	// internal failure information that should be preserved across restarts
	codeExecutionFork = 254
)

func makePrefix(code byte, keys ...interface{}) []byte {
	prefix := make([]byte, 1)
	prefix[0] = code
	for _, key := range keys {
		prefix = append(prefix, b(key)...)
	}
	return prefix
}

func b(v interface{}) []byte {
	switch i := v.(type) {
	case uint8:
		return []byte{i}
	case uint32:
		b := make([]byte, 4)
		binary.BigEndian.PutUint32(b, i)
		return b
	case uint64:
		b := make([]byte, 8)
		binary.BigEndian.PutUint64(b, i)
		return b
	case string:
		return []byte(i)
	case flow.Role:
		return []byte{byte(i)}
	case flow.Identifier:
		return i[:]
	case flow.ChainID:
		return []byte(i)
	default:
		panic(fmt.Sprintf("unsupported type to convert (%T)", v))
	}
}
