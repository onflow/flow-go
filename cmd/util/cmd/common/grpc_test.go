package common

import (
	"encoding/hex"
	"testing"

	"github.com/onflow/flow-go/utils/unittest"

	"github.com/stretchr/testify/require"
)

func TestGetGRPCDialOption(t *testing.T) {
	t.Run("should return valid secured GRPC connection when called with access-address and access-api-node-id", func(t *testing.T) {
		accessAddress := "17.123.255.123:2353"
		nk, err := unittest.NetworkingKey()
		require.NoError(t, err)
		accessApiNodePubKey := hex.EncodeToString(nk.PublicKey().Encode())
		insecureAccessAPI := false

		_, err = GetGRPCDialOption(accessAddress, accessApiNodePubKey, insecureAccessAPI)
		require.NoError(t, err)
	})

	t.Run("should return valid insecure GRPC connection when called with insecure-api true", func(t *testing.T) {
		accessAddress := "17.123.255.123:2353"
		accessApiNodePubKey := ""
		insecureAccessAPI := true

		_, err := GetGRPCDialOption(accessAddress, accessApiNodePubKey, insecureAccessAPI)
		require.NoError(t, err)
	})

	t.Run("should return error when called with invalid access address", func(t *testing.T) {
		accessAddress := ""
		accessApiNodePubKey := "02880abb813f1646952edb0a919d60444ebb34b92ce53e00868d526b80cf3621"
		insecureAccessAPI := false

		_, err := GetGRPCDialOption(accessAddress, accessApiNodePubKey, insecureAccessAPI)
		require.Error(t, err)
	})

	t.Run("should return error when called with invalid access api node ID", func(t *testing.T) {
		accessAddress := "17.123.255.123:2353"
		accessApiNodePubKey := ""
		insecureAccessAPI := false

		_, err := GetGRPCDialOption(accessAddress, accessApiNodePubKey, insecureAccessAPI)
		require.Error(t, err)
	})
}
