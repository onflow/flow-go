package types

import (
	"fmt"

	"github.com/onflow/cadence"
	gethCommon "github.com/onflow/go-ethereum/common"
)

// cadenceArrayTypeOfUInt8 is the Cadence type [UInt8]
var cadenceArrayTypeOfUInt8 = cadence.NewVariableSizedArrayType(cadence.UInt8Type)

// BytesToCadenceUInt8ArrayValue converts bytes into a Cadence array of type UInt8
func BytesToCadenceUInt8ArrayValue(b []byte) cadence.Array {
	values := make([]cadence.Value, len(b))
	for i, v := range b {
		values[i] = cadence.NewUInt8(v)
	}
	return cadence.NewArray(values).
		WithType(cadenceArrayTypeOfUInt8)
}

// cadenceHashType is the Cadence type [UInt8;32]
var cadenceHashType = cadence.NewConstantSizedArrayType(gethCommon.HashLength, cadence.UInt8Type)

// HashToCadenceArrayValue EVM hash ([32]byte) into a Cadence array of type [UInt8;32]
func HashToCadenceArrayValue(hash gethCommon.Hash) cadence.Array {
	values := make([]cadence.Value, len(hash))
	for i, v := range hash {
		values[i] = cadence.NewUInt8(v)
	}
	return cadence.NewArray(values).
		WithType(cadenceHashType)
}

// CadenceUInt8ArrayValueToBytes converts a Cadence array of type [UInt8] into a byte slice ([]byte)
func CadenceUInt8ArrayValueToBytes(a cadence.Value) ([]byte, error) {
	aa, ok := a.(cadence.Array)
	if !ok {
		return nil, fmt.Errorf("value is not an array")
	}

	arrayType := aa.Type()
	// if array type is empty, continue
	if arrayType != nil && !arrayType.Equal(cadenceArrayTypeOfUInt8) {
		return nil, fmt.Errorf("invalid array type")
	}

	values := make([]byte, len(aa.Values))
	for i, v := range aa.Values {
		values[i] = byte(v.(cadence.UInt8))
	}
	return values, nil
}
