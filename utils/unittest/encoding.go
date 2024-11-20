package unittest

import (
	"testing"

	"github.com/fxamacker/cbor/v2"
	"github.com/stretchr/testify/require"
)

// TODO doc
func CopyStructure(t *testing.T, src, dst any) {
	bz, err := cbor.Marshal(src)
	require.NoError(t, err)
	err = cbor.Unmarshal(bz, dst)
	require.NoError(t, err)
}
