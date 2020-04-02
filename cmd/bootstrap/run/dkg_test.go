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

	require.Len(t, data.Participants, 4)
}
