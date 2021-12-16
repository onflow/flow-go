package signature

import (
	"crypto/rand"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func randomByteSliceT(t *testing.T, len uint) []byte {
	msg, err := randomByteSlice(len)
	require.NoError(t, err)
	return msg
}

func randomByteSlice(len uint) ([]byte, error) {
	msg := make([]byte, len)
	n, err := rand.Read(msg)
	if err != nil {
		return nil, err
	}
	if uint(n) < len {
		return nil, fmt.Errorf("insufficient random bytes")
	}
	return msg, nil
}
