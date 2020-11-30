package ledger

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func Test_Keys_StartsWith(t *testing.T) {

	keyZero := NewKey(nil)

	keyA := NewKey([]KeyPart{
		NewKeyPart(0, []byte("abc")),
		NewKeyPart(0, []byte("def")),
	})

	keyAA := NewKey([]KeyPart{
		NewKeyPart(1, []byte("abc")),
		NewKeyPart(0, []byte("def")),
	})

	keyB := NewKey([]KeyPart{
		NewKeyPart(0, []byte("abc")),
		NewKeyPart(0, []byte("def")),
		NewKeyPart(0, []byte("ghi")),
	})

	require.True(t, keyB.StartsWith(keyA))
	require.False(t, keyA.StartsWith(keyB))
	require.True(t, keyA.StartsWith(keyA))
	require.True(t, keyA.StartsWith(keyZero))
	require.False(t, keyZero.StartsWith(keyA))

	require.False(t, keyA.StartsWith(keyAA))
	require.False(t, keyAA.StartsWith(keyA))
}
