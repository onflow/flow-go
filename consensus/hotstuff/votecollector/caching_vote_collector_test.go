package votecollector

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestCachingVoteCollector_AddVote tests that AddVote adds only unique votes and
// rejects duplicated votes.
func TestCachingVoteCollector_AddVote(t *testing.T) {
	t.Parallel()
	blockID := unittest.IdentifierFixture()
	t.Run("add-invalid-vote", func(t *testing.T) {
		collector := NewCachingVoteCollector(NewBaseVoteCollector(blockID))
		vote := unittest.VoteFixture()
		err := collector.AddVote(vote)
		require.Error(t, err)
		require.Empty(t, collector.GetVotes())
	})
	t.Run("add-valid-vote", func(t *testing.T) {
		collector := NewCachingVoteCollector(NewBaseVoteCollector(blockID))
		vote := unittest.VoteFixture()
		vote.BlockID = blockID
		err := collector.AddVote(vote)
		require.NoError(t, err)
		require.Equal(t, []*model.Vote{vote}, collector.GetVotes())
	})
	t.Run("add-duplicated-vote", func(t *testing.T) {
		collector := NewCachingVoteCollector(NewBaseVoteCollector(blockID))
		vote := unittest.VoteFixture()
		vote.BlockID = blockID
		err := collector.AddVote(vote)
		require.NoError(t, err)

		err = collector.AddVote(vote)
		require.NoError(t, err)
		require.Equal(t, []*model.Vote{vote}, collector.GetVotes())
	})
}

// TestCachingVoteCollector_ProcessingStatus tests that processing status is expected
func TestCachingVoteCollector_ProcessingStatus(t *testing.T) {
	t.Parallel()
	collector := NewCachingVoteCollector(NewBaseVoteCollector(unittest.IdentifierFixture()))
	require.Equal(t, hotstuff.CachingVotes, collector.ProcessingStatus())
}

// TestCachingVoteCollector_BlockID tests that blockID is expected
func TestCachingVoteCollector_BlockID(t *testing.T) {
	t.Parallel()
	blockID := unittest.IdentifierFixture()
	collector := NewCachingVoteCollector(NewBaseVoteCollector(blockID))
	require.Equal(t, blockID, collector.BlockID())
}

// TestCachingVoteCollector_GetVotes tests that GetVotes returns all previously cached votes
func TestCachingVoteCollector_GetVotes(t *testing.T) {
	t.Parallel()
	blockID := unittest.IdentifierFixture()
	collector := NewCachingVoteCollector(NewBaseVoteCollector(blockID))
	expectedVotes := make([]*model.Vote, 5)
	for i := range expectedVotes {
		vote := unittest.VoteFixture()
		vote.BlockID = blockID
		expectedVotes[i] = vote
		err := collector.AddVote(vote)
		require.NoError(t, err)
	}
	require.ElementsMatch(t, expectedVotes, collector.GetVotes())
}
