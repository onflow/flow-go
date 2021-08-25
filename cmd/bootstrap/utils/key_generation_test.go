package utils

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	sdkcrypto "github.com/onflow/flow-go-sdk/crypto"
	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/model/bootstrap"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestGenerateUnstakedNetworkingKey(t *testing.T) {

	key, err := GenerateUnstakedNetworkingKey(unittest.SeedFixture(crypto.KeyGenSeedMinLenECDSASecp256k1))
	require.NoError(t, err)
	assert.Equal(t, crypto.ECDSASecp256k1, key.Algorithm())
	assert.Equal(t, X962_NO_INVERSION, key.PublicKey().EncodeCompressed()[0])

	keys, err := GenerateUnstakedNetworkingKeys(20, unittest.SeedFixtures(20, crypto.KeyGenSeedMinLenECDSASecp256k1))
	require.NoError(t, err)
	for _, key := range keys {
		assert.Equal(t, crypto.ECDSASecp256k1, key.Algorithm())
		assert.Equal(t, X962_NO_INVERSION, key.PublicKey().EncodeCompressed()[0])
	}

}

func TestGenerateKeys(t *testing.T) {
	_, err := GenerateKeys(crypto.BLSBLS12381, 0, unittest.SeedFixtures(2, crypto.KeyGenSeedMinLenBLSBLS12381))
	require.EqualError(t, err, "n needs to match the number of seeds (0 != 2)")

	_, err = GenerateKeys(crypto.BLSBLS12381, 3, unittest.SeedFixtures(2, crypto.KeyGenSeedMinLenBLSBLS12381))
	require.EqualError(t, err, "n needs to match the number of seeds (3 != 2)")

	keys, err := GenerateKeys(crypto.BLSBLS12381, 2, unittest.SeedFixtures(2, crypto.KeyGenSeedMinLenBLSBLS12381))
	require.NoError(t, err)
	require.Len(t, keys, 2)
}

func TestGenerateStakingKeys(t *testing.T) {
	keys, err := GenerateStakingKeys(2, unittest.SeedFixtures(2, crypto.KeyGenSeedMinLenBLSBLS12381))
	require.NoError(t, err)
	require.Len(t, keys, 2)
}

func TestWriteMachineAccountFiles(t *testing.T) {
	nodes := append(
		unittest.PrivateNodeInfosFixture(5, unittest.WithRole(flow.RoleConsensus)),
		unittest.PrivateNodeInfosFixture(5, unittest.WithRole(flow.RoleCollection))...,
	)

	chainID := flow.Localnet
	chain := chainID.Chain()

	nodeIDLookup := make(map[string]flow.Identifier)
	expected := make(map[string]bootstrap.NodeMachineAccountInfo)
	for i, node := range nodes {
		// See comments in WriteMachineAccountFiles for why addresses take this form
		addr, err := chain.AddressAtIndex(uint64(6 + i*2))
		require.NoError(t, err)
		private, err := node.Private()
		require.NoError(t, err)

		expected[addr.HexWithPrefix()] = bootstrap.NodeMachineAccountInfo{
			Address:           addr.HexWithPrefix(),
			EncodedPrivateKey: private.NetworkPrivKey.Encode(),
			KeyIndex:          0,
			SigningAlgorithm:  private.NetworkPrivKey.Algorithm(),
			HashAlgorithm:     sdkcrypto.SHA3_256,
		}
		nodeIDLookup[addr.HexWithPrefix()] = node.NodeID
	}

	write := func(path string, value interface{}) error {
		actual, ok := value.(bootstrap.NodeMachineAccountInfo)
		require.True(t, ok)

		expectedInfo, ok := expected[actual.Address]
		require.True(t, ok)
		nodeID, ok := nodeIDLookup[actual.Address]
		require.True(t, ok)
		expectedPath := fmt.Sprintf(bootstrap.PathNodeMachineAccountInfoPriv, nodeID)

		assert.Equal(t, expectedPath, path)
		assert.Equal(t, expectedInfo, actual)
		// remove the value from the mapping, this ensures each one is passed
		// to the write function exactly once
		delete(expected, actual.Address)

		return nil
	}

	err := WriteMachineAccountFiles(chainID, nodes, write)
	require.NoError(t, err)
	assert.Len(t, expected, 0)
}

func TestWriteStakingNetworkingKeyFiles(t *testing.T) {
	nodes := unittest.PrivateNodeInfosFixture(20, unittest.WithAllRoles())

	// track expected calls to the write func
	expected := make(map[flow.Identifier]bootstrap.NodeInfoPriv)
	for _, node := range nodes {
		private, err := node.Private()
		require.NoError(t, err)
		expected[node.NodeID] = private
	}

	// check that the correct path and value are passed to the write function
	write := func(path string, value interface{}) error {
		actual, ok := value.(bootstrap.NodeInfoPriv)
		require.True(t, ok)

		expectedInfo, ok := expected[actual.NodeID]
		require.True(t, ok)
		expectedPath := fmt.Sprintf(bootstrap.PathNodeInfoPriv, expectedInfo.NodeID)

		assert.Equal(t, expectedPath, path)
		assert.Equal(t, expectedInfo, actual)
		// remove the value from the mapping, this ensures each one is passed
		// to the write function exactly once
		delete(expected, expectedInfo.NodeID)

		return nil
	}

	err := WriteStakingNetworkingKeyFiles(nodes, write)
	require.NoError(t, err)
	assert.Len(t, expected, 0)
}
