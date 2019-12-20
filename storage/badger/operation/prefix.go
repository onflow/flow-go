// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package operation

import (
	"encoding/binary"
	"fmt"

	"github.com/dapperlabs/flow-go/crypto"
	"github.com/dapperlabs/flow-go/model/flow"
)

const (
	codeRole        = 10
	codeAddress     = 11
	codeDelta       = 12
	codeHeader      = 20
	codeIdentities  = 21
	codeTransaction = 22
	codeCollections = 23
	codeHash        = 100 //block number to hash
	codeBoundary    = 101
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
	case uint64:
		b := make([]byte, 8)
		binary.BigEndian.PutUint64(b, i)
		return b
	case crypto.Hash:
		return []byte(i)
	case flow.Fingerprint:
		return []byte(i)
	case flow.Role:
		return []byte{byte(i)}
	case flow.Identifier:
		return i[:]
	default:
		panic(fmt.Sprintf("unsupported type to convert (%T)", v))
	}
}
