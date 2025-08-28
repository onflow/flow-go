package operation

import (
	op "github.com/onflow/flow-go/storage/operation"
)

const (

	// codes for special database markers
	codeMax    = 1 // keeps track of the maximum key size
	codeDBType = 2 // specifies a database type

	// codes for views with special meaning
	codeBeaconPrivateKey = 63 // BeaconPrivateKey, keyed by epoch counter
	codeDKGState         = 66 // current state of Recoverable Random Beacon State Machine for given epoch
)

func makePrefix(code byte, keys ...any) []byte {
	return op.MakePrefix(code, keys...)
}

func keyPartToBinary(v any) []byte {
	return op.AppendPrefixKeyPart(nil, v)
}
