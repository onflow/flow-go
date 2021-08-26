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
	block := unittest.BlockHeaderFixture()
	blockID := block.ID()
	t.Run("add-invalid-vote", func(t *testing.T) {
		collector := NewCachingVoteCollector(NewCollectionBase(block.View))
		vote := unittest.VoteFixture()
		err := collector.AddVote(vote)
		require.Error(t, err)
		require.Empty(t, collector.GetVotesByBlockID(blockID))
	})
	t.Run("add-valid-vote", func(t *testing.T) {
		collector := NewCachingVoteCollector(NewCollectionBase(block.View))
		vote := unittest.VoteFixture(func(vote *model.Vote) {
			vote.BlockID = blockID
			vote.View = block.View
		})
		err := collector.AddVote(vote)
		require.NoError(t, err)
		require.Equal(t, []*model.Vote{vote}, collector.GetVotesByBlockID(blockID))
	})
	t.Run("add-duplicated-vote", func(t *testing.T) {
		collector := NewCachingVoteCollector(NewCollectionBase(block.View))
		vote := unittest.VoteFixture(func(vote *model.Vote) {
			vote.BlockID = blockID
			vote.View = block.View
		})
		err := collector.AddVote(vote)
		require.NoError(t, err)

		err = collector.AddVote(vote)
		require.NoError(t, err)
		require.Equal(t, []*model.Vote{vote}, collector.GetVotesByBlockID(blockID))
	})
}

// TestCachingVoteCollector_ProcessingStatus tests that processing status is expected
func TestCachingVoteCollector_ProcessingStatus(t *testing.T) {
	t.Parallel()
	collector := NewCachingVoteCollector(NewCollectionBase(100))
	require.Equal(t, hotstuff.VoteCollectorStatus(hotstuff.VoteCollectorStatusCaching), collector.Status())
}

// TestCachingVoteCollector_BlockID tests that blockID is expected
func TestCachingVoteCollector_BlockID(t *testing.T) {
	t.Parallel()
	block := unittest.BlockHeaderFixture()
	collector := NewCachingVoteCollector(NewCollectionBase(block.View))
	require.Equal(t, block.View, collector.View())
}

// TestCachingVoteCollector_GetVotes tests that GetVotes returns all previously cached votes
func TestCachingVoteCollector_GetVotes(t *testing.T) {
	t.Parallel()
	block := unittest.BlockHeaderFixture()
	blockID := block.ID()
	collector := NewCachingVoteCollector(NewCollectionBase(block.View))
	expectedVotes := make([]*model.Vote, 5)
	for i := range expectedVotes {
		vote := unittest.VoteFixture(func(vote *model.Vote) {
			vote.BlockID = blockID
			vote.View = block.View
		})
		expectedVotes[i] = vote
		err := collector.AddVote(vote)
		require.NoError(t, err)
	}
	require.ElementsMatch(t, expectedVotes, collector.GetVotesByBlockID(blockID))
}
