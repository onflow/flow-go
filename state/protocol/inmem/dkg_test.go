package inmem_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/state/protocol/inmem"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestDKGv0 tests that the [inmem.DKG] is backward compatible with the v0 DKG protocol model.
// We test this by creating a [inmem.DKG] instance from a v0 [flow.EpochSetup] and [flow.EpochCommit]
// and verifying that the DKG methods return the expected values.
// TODO(EFM, #6794): Remove this once we complete the network upgrade
func TestDKGv0(t *testing.T) {
	consensusParticipants := unittest.IdentityListFixture(5, unittest.WithRole(flow.RoleConsensus)).Sort(flow.Canonical[flow.Identity])
	otherParticipants := unittest.IdentityListFixture(10, unittest.WithAllRolesExcept(flow.RoleConsensus))
	setup := unittest.EpochSetupFixture(unittest.WithParticipants(append(consensusParticipants, otherParticipants...).ToSkeleton()))
	commit := unittest.EpochCommitFixture(unittest.WithDKGFromParticipants(setup.Participants))
	dkg := inmem.NewDKGv0(setup, commit)
	t.Run("Index", func(t *testing.T) {
		for i, participant := range consensusParticipants {
			index, err := dkg.Index(participant.NodeID)
			require.NoError(t, err)
			require.Equal(t, uint(i), index)
		}
		_, err := dkg.Index(otherParticipants[0].NodeID)
		require.Error(t, err)
		require.True(t, protocol.IsIdentityNotFound(err))
	})
	t.Run("NodeID", func(t *testing.T) {
		for i, participant := range consensusParticipants {
			nodeID, err := dkg.NodeID(uint(i))
			require.NoError(t, err)
			require.Equal(t, participant.NodeID, nodeID)
		}
		_, err := dkg.NodeID(uint(len(consensusParticipants)))
		require.Error(t, err)
	})
	t.Run("KeyShare", func(t *testing.T) {
		for i, participant := range consensusParticipants {
			keyShare, err := dkg.KeyShare(participant.NodeID)
			require.NoError(t, err)
			require.Equal(t, commit.DKGParticipantKeys[i], keyShare)
		}
		_, err := dkg.KeyShare(otherParticipants[0].NodeID)
		require.Error(t, err)
		require.True(t, protocol.IsIdentityNotFound(err))
	})
	t.Run("Size", func(t *testing.T) {
		require.Equal(t, uint(len(consensusParticipants)), dkg.Size())
	})
	t.Run("GroupKey", func(t *testing.T) {
		require.Equal(t, commit.DKGGroupKey, dkg.GroupKey())
	})
}
