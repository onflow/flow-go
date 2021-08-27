package grpcutils

import (
	"encoding/hex"
	"github.com/onflow/flow-go/utils/unittest"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestSecureGRPCDialOpt(t *testing.T) {
	t.Run("should return valid secured GRPC dial option with no errors", func(t *testing.T) {
		nk, err := unittest.NetworkingKey()
		pk := hex.EncodeToString(nk.PublicKey().Encode())
		_, err = SecureGRPCDialOpt(pk)
		require.NoError(t, err)
	})

	t.Run("should return error when invalid public key argument used", func(t *testing.T) {
		nk, err := unittest.NetworkingKey()

		// un-encoded public key will cause hex decoding to fail
		_, err = SecureGRPCDialOpt(nk.PublicKey().String())
		require.NoError(t, err)
	})
}
