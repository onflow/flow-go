package dkg

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestRunDKG(t *testing.T) {
	seedLen := crypto.SeedMinLenDKG
	_, err := RunDKG(0, unittest.SeedFixtures(2, seedLen))
	require.EqualError(t, err, "n needs to match the number of seeds (0 != 2)")

	_, err = RunDKG(3, unittest.SeedFixtures(2, seedLen))
	require.EqualError(t, err, "n needs to match the number of seeds (3 != 2)")

	data, err := RunDKG(4, unittest.SeedFixtures(4, seedLen))
	require.NoError(t, err)

	require.Len(t, data.PrivKeyShares, 4)
	require.Len(t, data.PubKeyShares, 4)
}
