package run

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/utils/unittest"
)

func TestRunDKG(t *testing.T) {
	_, err := RunDKG(0, unittest.SeedFixtures(2, 16))
	require.EqualError(t, err, "n needs to match the number of seeds (0 != 2)")

	_, err = RunDKG(3, unittest.SeedFixtures(2, 16))
	require.EqualError(t, err, "n needs to match the number of seeds (3 != 2)")

	data, err := RunDKG(4, unittest.SeedFixtures(4, 16))
	require.NoError(t, err)

	for i, p := range data.Participants {
		expected, err := p.Priv.PublicKey().Encode()
		require.NoError(t, err)

		priv, err := p.Priv.Encode()
		require.NoError(t, err)
		require.NotEmpty(t, priv)

		pub, err := data.PubKeys[i].Encode()
		require.NoError(t, err)
		require.NotEmpty(t, pub)

		require.Equal(t, expected, pub)
	}

	require.Len(t, data.Participants, 4)
}
