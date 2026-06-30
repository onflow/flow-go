package events

import (
	gethCommon "github.com/ethereum/go-ethereum/common"
	"github.com/onflow/cadence"

	"github.com/onflow/flow-go/fvm/evm/types"
)

// cadenceArrayTypeOfUInt8 is the Cadence type [UInt8]
var cadenceArrayTypeOfUInt8 = cadence.NewVariableSizedArrayType(cadence.UInt8Type)

// bytesToCadenceUInt8ArrayValue converts bytes into a Cadence array of type UInt8
func bytesToCadenceUInt8ArrayValue(b []byte) cadence.Array {
	values := make([]cadence.Value, len(b))
	for i, v := range b {
		values[i] = cadence.NewUInt8(v)
	}
	return cadence.NewArray(values).
		WithType(cadenceArrayTypeOfUInt8)
}

// cadenceHashType is the Cadence type [UInt8;32]
var cadenceHashType = cadence.NewConstantSizedArrayType(gethCommon.HashLength, cadence.UInt8Type)

// hashToCadenceArrayValue EVM hash ([32]byte) into a Cadence array of type [UInt8;32]
func hashToCadenceArrayValue(hash gethCommon.Hash) cadence.Array {
	values := make([]cadence.Value, len(hash))
	for i, v := range hash {
		values[i] = cadence.NewUInt8(v)
	}
	return cadence.NewArray(values).
		WithType(cadenceHashType)
}

// checksumType is the Cadence type [UInt8;4]
var checksumType = cadence.NewConstantSizedArrayType(types.ChecksumLength, cadence.UInt8Type)

// checksumToCadenceArrayValue converts a checksum ([4]byte) into a Cadence array of type [UInt8;4]
func checksumToCadenceArrayValue(checksum [types.ChecksumLength]byte) cadence.Array {
	values := make([]cadence.Value, types.ChecksumLength)
	for i := range types.ChecksumLength {
		values[i] = cadence.NewUInt8(checksum[i])
	}
	return cadence.NewArray(values).
		WithType(checksumType)
}
