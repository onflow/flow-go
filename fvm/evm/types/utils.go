package types

import (
	"fmt"

	"github.com/onflow/cadence"
)

// BytesToCadenceUInt8ArrayValue converts bytes into a Cadence array of type UInt8
func BytesToCadenceUInt8ArrayValue(b []byte) cadence.Array {
	values := make([]cadence.Value, len(b))
	for i, v := range b {
		values[i] = cadence.NewUInt8(v)
	}
	return cadence.NewArray(values).WithType(
		cadence.NewVariableSizedArrayType(cadence.UInt8Type),
	)
}

var cadenceArrayTypeOfUInt8 = cadence.NewVariableSizedArrayType(cadence.UInt8Type)

// CadenceUInt8ArrayValueToBytes converts a Cadence array of type UInt8 into a byte slice
func CadenceUInt8ArrayValueToBytes(a cadence.Array) ([]byte, error) {
	if !a.ArrayType.Equal(cadenceArrayTypeOfUInt8) {
		return nil, fmt.Errorf("invalid array type")
	}
	values := make([]byte, len(a.Values))
	for i, v := range a.Values {
		values[i] = byte(v.(cadence.UInt8))
	}
	return values, nil
}
