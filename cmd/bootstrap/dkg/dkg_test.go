package dkg

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/crypto"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestBeaconKG(t *testing.T) {
	seed := unittest.SeedFixture(2 * crypto.KeyGenSeedMinLen)

	// n = 0
	_, err := RandomBeaconKG(0, seed)
	require.EqualError(t, err, "Beacon KeyGen failed: size should be between 2 and 254, got 0")

	// should work for case n = 1
	_, err = RandomBeaconKG(1, seed)
	require.NoError(t, err)

	// n = 4
	data, err := RandomBeaconKG(4, seed)
	require.NoError(t, err)
	require.Len(t, data.PrivKeyShares, 4)
	require.Len(t, data.PubKeyShares, 4)
}
