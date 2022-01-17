package dkg

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestGetDKGCommitteeIndex(t *testing.T) {
	t.Run("should return error if nodeID not found in participant list", func(t *testing.T) {
		participants := unittest.IdentityListFixture(3, unittest.WithRole(flow.RoleConsensus))
		_, err := GetDKGCommitteeIndex(unittest.IdentifierFixture(), participants)
		require.Error(t, err)
	})

	t.Run("should return index of nodeID in participant list", func(t *testing.T) {
		participants := unittest.IdentityListFixture(3, unittest.WithRole(flow.RoleConsensus))
		participantIndex, err := GetDKGCommitteeIndex(participants[0].NodeID, participants)
		require.NoError(t, err)
		require.Equal(t, 0, participantIndex)
	})
}
