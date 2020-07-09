package ledger_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/ledger"
)

// Test_PayloadEncodingDecoding tests encoding decoding functionality of a payload
func Test_PayloadEncodingDecoding(t *testing.T) {
	kp1t := uint16(1)
	kp1v := []byte("key part 1")
	kp1 := ledger.NewKeyPart(kp1t, kp1v)

	kp2t := uint16(22)
	kp2v := []byte("key part 2")
	kp2 := ledger.NewKeyPart(kp2t, kp2v)

	k := ledger.NewKey([]ledger.KeyPart{*kp1, *kp2})

	v := ledger.Value([]byte{'A'})

	p := ledger.NewPayload(*k, v)

	encoded := p.Encode()
	newp, err := ledger.DecodePayload(encoded)
	require.NoError(t, err)
	require.Equal(t, newp.Key.KeyParts[0].Type, kp1t)
	require.Equal(t, newp.Key.KeyParts[0].Value, kp1v)
	require.Equal(t, newp.Key.KeyParts[1].Type, kp2t)
	require.Equal(t, newp.Key.KeyParts[1].Value, kp2v)
	require.Equal(t, newp.Value, v)
}
