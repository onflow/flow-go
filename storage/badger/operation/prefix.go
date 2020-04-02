// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package operation

import (
	"encoding/binary"
	"fmt"

	"github.com/dapperlabs/flow-go/model/flow"
)

const (

	// special database markers
	codeBoundary = 1 // latest finalized block number
	codeNumber   = 2 // lookup for block by number
	codeDelta    = 3 // history of stake changes

	// block header and entities included in block contents
	codeHeader    = 10
	codeIdentity  = 11
	codeGuarantee = 12
	codeSeal      = 13

	// entities that are related to block formation & validation
	codeTransaction     = 21
	codeCollection      = 22
	codeCommit          = 23
	codeExecutionResult = 24
	// codeReceipt       = 25
	// codeApproval      = 26
	codeChunkHeader        = 27
	codeChunkDataPack      = 28
	codeEvent              = 29
	codeExecutionStateView = 30

	codeIndexIdentity                = 100
	codeIndexGuarantee               = 101
	codeIndexSeal                    = 102
	codeIndexCollection              = 104
	codeIndexSealByBlock             = 105
	codeIndexExecutionResultByBlock  = 106
	codeIndexCollectionByTransaction = 107
	codeIndexHeaderByCollection      = 108
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
