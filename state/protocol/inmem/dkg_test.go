package inmem_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/state/protocol/inmem"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestDKG tests the DKG implementation with the new DKG protocol model.
func TestDKG(t *testing.T) {
	consensusParticipants := unittest.IdentityListFixture(5, unittest.WithRole(flow.RoleConsensus)).Sort(flow.Canonical[flow.Identity])
	otherParticipants := unittest.IdentityListFixture(10, unittest.WithAllRolesExcept(flow.RoleConsensus))
	setup := unittest.EpochSetupFixture(unittest.WithParticipants(append(consensusParticipants, otherParticipants...).ToSkeleton()))
	commit := unittest.EpochCommitFixture(unittest.WithDKGFromParticipants(setup.Participants))
	dkg := inmem.NewDKG(setup, commit)
	t.Run("Index", func(t *testing.T) {
		for _, participant := range consensusParticipants {
			_, err := dkg.Index(participant.NodeID)
			require.NoError(t, err)
		}
		_, err := dkg.Index(otherParticipants[0].NodeID)
		require.Error(t, err)
		require.True(t, protocol.IsIdentityNotFound(err))
	})
	t.Run("NodeID", func(t *testing.T) {
		for i := uint(0); i < uint(len(consensusParticipants)); i++ {
			_, err := dkg.NodeID(i)
			require.NoError(t, err)
		}
		_, err := dkg.NodeID(uint(len(consensusParticipants)))
		require.Error(t, err)
	})
	t.Run("KeyShare", func(t *testing.T) {
		for _, participant := range consensusParticipants {
			_, err := dkg.KeyShare(participant.NodeID)
			require.NoError(t, err)
		}
		_, err := dkg.KeyShare(otherParticipants[0].NodeID)
		require.Error(t, err)
		require.True(t, protocol.IsIdentityNotFound(err))
	})
	t.Run("Size", func(t *testing.T) {
		require.Equal(t, uint(len(commit.DKGParticipantKeys)), dkg.Size())
	})
	t.Run("GroupKey", func(t *testing.T) {
		require.Equal(t, commit.DKGGroupKey, dkg.GroupKey())
	})
}
