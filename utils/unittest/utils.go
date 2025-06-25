package unittest

import (
	"encoding/json"
	"testing"

	"github.com/onflow/cadence"
	jsoncdc "github.com/onflow/cadence/encoding/json"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// VerifyCdcArguments verifies that the actual slice of Go values match the expected set of Cadence values.
func VerifyCdcArguments(t *testing.T, expected []cadence.Value, actual []interface{}) {
	for index, arg := range actual {
		// marshal to bytes
		bz, err := json.Marshal(arg)
		require.NoError(t, err)

		// parse cadence value
		decoded, err := jsoncdc.Decode(nil, bz)
		require.NoError(t, err)

		assert.Equal(t, expected[index], decoded)
	}
}

// InterfaceToCdcValues decodes jsoncdc encoded values from interface -> cadence value.
func InterfaceToCdcValues(t *testing.T, vals []interface{}) []cadence.Value {
	decoded := make([]cadence.Value, len(vals))
	for index, val := range vals {
		// marshal to bytes
		bz, err := json.Marshal(val)
		require.NoError(t, err)

		// parse cadence value
		cdcVal, err := jsoncdc.Decode(nil, bz)
		require.NoError(t, err)

		decoded[index] = cdcVal
	}

	return decoded
}

// BytesToCdcUInt8 converts a Go []byte to a Cadence []UInt8 with equal content.
func BytesToCdcUInt8(data []byte) []cadence.Value {
	ret := make([]cadence.Value, len(data))
	for i, v := range data {
		ret[i] = cadence.UInt8(v)
	}
	return ret
}
