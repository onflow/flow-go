// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package operation

import (
	"encoding/binary"
	"fmt"

	"github.com/dapperlabs/flow-go/model/flow"
)

const (

	// codes for special database markers
	codeMax = 1 // keeps track of the maximum key size

	// codes for views with special meaning
	codeStartedView = 10 // latest view hotstuff started
	codeVotedView   = 11 // latest view hotstuff voted on

	// code for heights with special meaning
	codeFinalizedHeight = 20 // latest finalized block height
	codeSealedHeight    = 21 // latest sealed block height
	codeClusterHeight   = 22 // latest finalized height on cluster
	codeExecutedBlock   = 23 // latest executed block with max height

	// codes for single entity storage
	codeHeader          = 30
	codeIdentity        = 31
	codeGuarantee       = 32
	codeSeal            = 33
	codeTransaction     = 34
	codeCollection      = 35
	codeExecutionResult = 36

	// codes for indexing single identifier by identifier
	codeHeightToBlock       = 40 // index mapping height to block ID
	codeBlockToSeal         = 41 // index mapping a block its last payload seal
	codeCollectionReference = 42 // index reference block ID for collection

	// codes for indexing multiple identifiers by identifier
	codeBlockChildren     = 50 // index mapping block ID to children blocks
	codePayloadIdentities = 51 // index mapping block ID to payload identities
	codePayloadGuarantees = 52 // index mapping block ID to payload guarantees
	codePayloadSeals      = 53 // index mapping block ID to payload seals
	codeCollectionBlock   = 54 // index mapping collection ID to block ID

	// legacy codes (should be cleaned up)
	codeChunkDataPack                = 100
	codeCommit                       = 101
	codeEvent                        = 102
	codeExecutionStateInteractions   = 103
	codeTransactionResult            = 104
	codeFinalizedCluster             = 105
	codeIndexCollection              = 200
	codeIndexExecutionResultByBlock  = 202
	codeIndexCollectionByTransaction = 203
	codeIndexCollectionFinalized     = 204
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
	default:
		panic(fmt.Sprintf("unsupported type to convert (%T)", v))
	}
}
