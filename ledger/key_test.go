package ledger_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/ledger"
)

// Test_KeyEncodingDecoding tests encoding decoding functionality of a key
func Test_KeyEncodingDecoding(t *testing.T) {
	kp1t := uint16(1)
	kp1v := []byte("key part 1")
	kp1 := ledger.KeyPart{Type: kp1t, Value: kp1v}

	kp2t := uint16(22)
	kp2v := []byte("key part 2")
	kp2 := ledger.KeyPart{Type: kp2t, Value: kp2v}

	k := &ledger.Key{[]ledger.KeyPart{kp1, kp2}}

	encoded := k.Encode()
	newk := ledger.DecodeKey(encoded)
	require.Equal(t, newk.KeyParts[0].Type, kp1t)
	require.Equal(t, newk.KeyParts[0].Value, kp1v)

	require.Equal(t, newk.KeyParts[1].Type, kp2t)
	require.Equal(t, newk.KeyParts[1].Value, kp2v)
}
