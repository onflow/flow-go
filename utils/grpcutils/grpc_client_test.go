package grpcutils

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/utils/unittest"
)

func TestSecureGRPCDialOpt(t *testing.T) {
	t.Run("should return valid secured GRPC dial option with no errors", func(t *testing.T) {
		nk := unittest.NetworkingPrivKeyFixture()
		_, err := SecureGRPCDialOpt(nk.PublicKey().String()[2:])
		require.NoError(t, err)
	})

	t.Run("should return error when invalid public key argument used", func(t *testing.T) {
		// un-encoded public key will cause hex decoding to fail
		_, err := SecureGRPCDialOpt("")
		require.Error(t, err)
	})
}
