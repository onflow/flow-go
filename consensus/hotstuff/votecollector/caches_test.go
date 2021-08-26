package votecollector

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestPendingVotes_AddVote tests that AddVote skips duplicated votes
func TestPendingVotes_AddVote(t *testing.T) {
	t.Parallel()

	votes := NewPendingVotes()
	vote := unittest.VoteFixture()

	require.True(t, votes.AddVote(vote))
	require.False(t, votes.AddVote(vote))
	require.Len(t, votes.All(), 1)
}

// TestPendingVotes_All tests that All returns previously added votes in same order
func TestPendingVotes_All(t *testing.T) {
	t.Parallel()

	votes := NewPendingVotes()
	expectedVotes := make([]*model.Vote, 0, 5)
	for i := range expectedVotes {
		vote := unittest.VoteFixture()
		expectedVotes[i] = vote
		_ = votes.AddVote(vote)
	}
	require.Equal(t, expectedVotes, votes.All())
}

// TestPendingVotes_ByBlockID returns previously added orders filtered by blockID
func TestPendingVotes_ByBlockID(t *testing.T) {
	votes := NewPendingVotes()
	expectedVotes := make([]*model.Vote, 0, 5)
	blockID := unittest.IdentifierFixture()
	for i := range expectedVotes {
		vote := unittest.VoteFixture(func(vote *model.Vote) {
			vote.BlockID = blockID
		})
		expectedVotes[i] = vote
		_ = votes.AddVote(vote)
	}

	for i := 0; i < 10; i++ {
		vote := unittest.VoteFixture()
		require.True(t, votes.AddVote(vote))
	}

	require.Equal(t, expectedVotes, votes.ByBlockID(blockID))
}
