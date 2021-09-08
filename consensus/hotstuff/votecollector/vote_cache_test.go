package votecollector

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestVotesCache_View tests that View returns same value that was set by constructor
func TestVotesCache_View(t *testing.T) {
	view := uint64(100)
	cache := NewVotesCache(view)
	require.Equal(t, view, cache.View())
}

// TestVotesCache_AddVoteRepeatedVote tests that AddVote skips duplicated votes
func TestVotesCache_AddVoteRepeatedVote(t *testing.T) {
	t.Parallel()

	view := uint64(100)
	cache := NewVotesCache(view)
	vote := unittest.VoteFixture(unittest.WithVoteView(view))

	require.NoError(t, cache.AddVote(vote))
	err := cache.AddVote(vote)
	require.ErrorIs(t, err, RepeatedVoteErr)
}

// TestVotesCache_AddVoteIncompatibleView tests that adding vote with incompatible view results in error
func TestVotesCache_AddVoteIncompatibleView(t *testing.T) {
	t.Parallel()

	view := uint64(100)
	cache := NewVotesCache(view)
	vote := unittest.VoteFixture(unittest.WithVoteView(view + 1))
	err := cache.AddVote(vote)
	require.ErrorIs(t, err, VoteForIncompatibleViewError)
}

// TestVotesCache_All tests that All returns previously added votes in same order
func TestVotesCache_All(t *testing.T) {
	t.Parallel()

	view := uint64(100)
	cache := NewVotesCache(view)
	expectedVotes := make([]*model.Vote, 0, 5)
	for i := range expectedVotes {
		vote := unittest.VoteFixture(unittest.WithVoteView(view))
		expectedVotes[i] = vote
		require.NoError(t, cache.AddVote(vote))
	}
	require.Equal(t, expectedVotes, cache.All())
}

// TestVotesCache_RegisterVoteConsumer tests that registered vote consumer receives all previously added votes as well as
// new ones in expected order.
func TestVotesCache_RegisterVoteConsumer(t *testing.T) {
	t.Parallel()

	view := uint64(100)
	cache := NewVotesCache(view)
	votesBatchSize := 5
	expectedVotes := make([]*model.Vote, 0, votesBatchSize)
	// produce first batch before registering vote consumer
	for i := range expectedVotes {
		vote := unittest.VoteFixture(unittest.WithVoteView(view))
		expectedVotes[i] = vote
		require.NoError(t, cache.AddVote(vote))
	}

	consumedVotes := make([]*model.Vote, 0)
	voteConsumer := func(vote *model.Vote) {
		consumedVotes = append(consumedVotes, vote)
	}

	// registering vote consumer has to fill consumedVotes using callback
	cache.RegisterVoteConsumer(voteConsumer)
	// all cached votes should be fed into the consumer right away
	require.Equal(t, expectedVotes, consumedVotes)

	// produce second batch after registering vote consumer
	for i := 0; i < votesBatchSize; i++ {
		vote := unittest.VoteFixture(unittest.WithVoteView(view))
		expectedVotes = append(expectedVotes, vote)
		require.NoError(t, cache.AddVote(vote))
	}

	// at this point consumedVotes has to have all votes created before and after registering vote
	// consumer, and they must be in same order
	require.Equal(t, expectedVotes, consumedVotes)
}
