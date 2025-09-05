package operation

import (
	op "github.com/onflow/flow-go/storage/operation"
)

const (

	// codes for special database markers
	codeMax    = 1 // keeps track of the maximum key size
	codeDBType = 2 // specifies a database type

	// codes for views with special meaning
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
