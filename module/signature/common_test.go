package signature

import (
	"crypto/rand"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func createMSGT(t *testing.T) []byte {
	msg, err := createMSG()
	require.NoError(t, err)
	return msg
}

func createMSGB(b *testing.B) []byte {
	msg, err := createMSG()
	if err != nil {
		b.Fatal(err)
	}
	return msg
}

func createMSG() ([]byte, error) {
	msg := make([]byte, 128)
	n, err := rand.Read(msg)
	if err != nil {
		return nil, err
	}
	if n < len(msg) {
		return nil, fmt.Errorf("insufficient random bytes")
	}
	return msg, nil
}
