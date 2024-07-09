package types_test

import (
	"testing"

	"github.com/onflow/cadence"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/fvm/evm/types"
)

func TestCadenceConversion(t *testing.T) {
	input := []byte{1, 2, 3, 4, 5}

	inCadence := types.BytesToCadenceUInt8ArrayValue(input)
	output, err := types.CadenceUInt8ArrayValueToBytes(inCadence)
	require.NoError(t, err)
	require.Equal(t, input, output)

	invalidTypeArray := cadence.NewArray(nil).
		WithType(cadence.NewVariableSizedArrayType(cadence.UFix64Type))
	_, err = types.CadenceUInt8ArrayValueToBytes(invalidTypeArray)
	require.Error(t, err)
}
