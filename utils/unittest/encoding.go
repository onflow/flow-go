package unittest

import (
	"testing"

	"github.com/fxamacker/cbor/v2"
	"github.com/stretchr/testify/require"
)

// CopyStructure copies public fields of compatible types from src to dst.
func CopyStructure(t *testing.T, src, dst any) {
	bz, err := cbor.Marshal(src)
	require.NoError(t, err)
	err = cbor.Unmarshal(bz, dst)
	require.NoError(t, err)
}
