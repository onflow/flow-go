package rlp_test

import (
	"testing"

	"github.com/onflow/go-ethereum/rlp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRLPStructFieldOrder tests the field ordering property of RLP encoding.
// It provides evidence that RLP encoding depends on struct field ordering.
func TestRLPStructFieldOrder(t *testing.T) {
	a := struct {
		A uint32 // A first
		B uint32
	}{A: 2, B: 3}

	b := struct {
		B uint32 // B first
		A uint32
	}{A: 2, B: 3}

	abin, err := rlp.EncodeToBytes(a)
	require.NoError(t, err)
	bbin, err := rlp.EncodeToBytes(b)
	require.NoError(t, err)
	assert.NotEqual(t, abin, bbin)
}
