package events

import (
	"github.com/onflow/cadence"
	gethCommon "github.com/onflow/go-ethereum/common"
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

// ChecksumLength captures number of bytes a checksum uses
const ChecksumLength = 4

// checksumType is the Cadence type [UInt8;4]
var checksumType = cadence.NewConstantSizedArrayType(ChecksumLength, cadence.UInt8Type)

// checksumToCadenceArrayValue converts a checksum ([4]byte) into a Cadence array of type [UInt8;4]
func checksumToCadenceArrayValue(checksum [ChecksumLength]byte) cadence.Array {
	values := make([]cadence.Value, ChecksumLength)
	for i := 0; i < ChecksumLength; i++ {
		values[i] = cadence.NewUInt8(checksum[i])
	}
	return cadence.NewArray(values).
		WithType(checksumType)
}
