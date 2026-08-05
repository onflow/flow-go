package unittest

import (
	"testing"

	"github.com/fxamacker/cbor/v2"
	"github.com/stretchr/testify/require"
)

// EncodeDecodeDifferentVersions emulates the situation where a peer running software version A receives
// a message from a sender running software version B, where the format of the message may have been upgraded between
// the different software versions. This method works irrespective whether version A or B is the older/newer version
// (also allowing that both versions are the same; in this degenerate edge-case the old and new format would be the same).
//
// This function works by encoding src using CBOR, then decoding the result into dst.
// Compatible fields as defined by CBOR will be copied into dst; in-compatible fields
// may be omitted.
func EncodeDecodeDifferentVersions(t *testing.T, src, dst any) {
	bz, err := cbor.Marshal(src)
	require.NoError(t, err)
	err = cbor.Unmarshal(bz, dst)
	require.NoError(t, err)
}

// PtrTo returns a pointer to the input. Useful for concisely constructing
// a reference-typed argument to a function or similar.
func PtrTo[T any](target T) *T {
	return &target
}
