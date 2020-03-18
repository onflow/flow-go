package run

import (
	"testing"

	"github.com/dapperlabs/flow-go/utils/unittest"
	"github.com/stretchr/testify/require"
)

func TestRunDKG(t *testing.T) {
	_, err := RunDKG(0, unittest.SeedFixtures(2, 16))
	require.EqualError(t, err, "n needs to match the number of seeds (0 != 2)")

	_, err = RunDKG(3, unittest.SeedFixtures(2, 16))
	require.EqualError(t, err, "n needs to match the number of seeds (3 != 2)")

	data, err := RunDKG(4, unittest.SeedFixtures(4, 16))
	require.NoError(t, err)

	for _, p := range data.Participants {
		bs, err := p.Priv.Encode()
		require.NoError(t, err)
		require.NotEmpty(t, bs)

		bs, err = p.Pub.Encode()
		require.NoError(t, err)
		require.NotEmpty(t, bs)
	}

	require.Len(t, data.Participants, 4)
}
