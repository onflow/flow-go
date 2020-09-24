package run

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/utils/unittest"
)

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
