package run

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/crypto"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

func TestGenerateKeys(t *testing.T) {
	_, err := GenerateKeys(crypto.BLS_BLS12381, 0, unittest.SeedFixtures(2, crypto.KeyGenSeedMinLenBlsBls12381))
	require.EqualError(t, err, "n needs to match the number of seeds (0 != 2)")

	_, err = GenerateKeys(crypto.BLS_BLS12381, 3, unittest.SeedFixtures(2, crypto.KeyGenSeedMinLenBlsBls12381))
	require.EqualError(t, err, "n needs to match the number of seeds (3 != 2)")

	keys, err := GenerateKeys(crypto.BLS_BLS12381, 2, unittest.SeedFixtures(2, crypto.KeyGenSeedMinLenBlsBls12381))
	require.NoError(t, err)
	require.Len(t, keys, 2)
}

func TestGenerateStakingKeys(t *testing.T) {
	keys, err := GenerateStakingKeys(2, unittest.SeedFixtures(2, crypto.KeyGenSeedMinLenBlsBls12381))
	require.NoError(t, err)
	require.Len(t, keys, 2)
}
