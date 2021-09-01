package votecollector

import (
	"errors"
	"sync"

	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/model/flow"
)

var (
	// RepeatedVoteErr is emitted, when we receive a vote for the same block
	// from the same voter multiple times. This error does _not_ indicate
	// equivocation.
	RepeatedVoteErr = errors.New("duplicated vote")
)

// voteContainer container stores the vote and in index representing
// the order in which the votes were received
type voteContainer struct {
	*model.Vote
	index int
}

// VoteConsumer is a callback for consuming votes. VotesCache feeds votes into
// the consumer in the order they are received. Only votes that pass de-
// duplication and equivocation detection are passed on.
// CAUTION: a consumer MUST be NON-BLOCKING and consume the votes without
// noteworthy delay. Otherwise, consensus speed is impacted.
type VoteConsumer func(vote *model.Vote)

// VotesCache maintains a cache of votes for one particular view. The cache
// memorizes the order in which the votes were received. Votes are
// de-duplicated based on the following rules:
//  * Vor each voter (i.e. SignerID), we store the _first_ vote v0.
//  * For any subsequent vote v, we check whether v.BlockID == v0.BlockID.
//    If this is the case, we consider the vote a duplicate and drop it.
//    If v and v0 have different BlockIDs, the voter is equivocating and
//    we return a model.DoubleVoteError
type VotesCache struct {
	lock          sync.Mutex
	view          uint64
	votes         map[flow.Identifier]voteContainer // signerID -> first vote
	voteConsumers []VoteConsumer
}

// NewVotesCache instantiates a VotesCache for the given view
func NewVotesCache(view uint64) *VotesCache {
	return &VotesCache{
		view:  view,
		votes: make(map[flow.Identifier]voteContainer),
	}
}

func (vc *VotesCache) View() uint64 { return vc.view }

// AddVote stores a vote in the cache. The following errors are expected during
// normal operations:
//  * nil: if the vote was successfully added
//  * model.DoubleVoteError is returned if the voter is equivocating
//    (i.e. voting in the same view for different blocks). Vote is not stored
//  * RepeatedVoteErr is returned when adding a vote for the same block from
//    the same voter multiple times. Vote is not stored.
func (vc *VotesCache) AddVote(vote *model.Vote) error {
	vc.lock.Lock()
	defer vc.lock.Unlock()

	// De-duplicated votes based on the following rules:
	//  * Vor each voter (i.e. SignerID), we store the _first_ vote v0.
	//  * For any subsequent vote v, we check whether v.BlockID == v0.BlockID.
	//    If this is the case, we consider the vote a duplicate and drop it.
	//    If v and v0 have different BlockIDs, the voter is equivocating and
	//    we return a model.DoubleVoteError
	firstVote, exists := vc.votes[vote.SignerID]
	if exists {
		if firstVote.BlockID != vote.BlockID {
			return model.NewDoubleVoteErrorf(firstVote.Vote, vote, "detected vote equivocation at view: %d", vc.view)
		}
		return RepeatedVoteErr
	}

	// previously unknown vote: (1) store and (2) forward to consumers
	vc.votes[vote.SignerID] = voteContainer{vote, len(vc.votes)}
	for _, consumer := range vc.voteConsumers {
		consumer(vote)
	}
	return nil
}

// RegisterVoteConsumer registers a VoteConsumer. Upon registration, the cache
// feeds all cached votes into the consumer in the order they arrived.
// CAUTION: a consumer _must_ be non-blocking and consume the votes without
// noteworthy delay. Otherwise, consensus speed is impacted.
//
// Expected usage patter: During happy-path operations, the block arrives in a
// timely manner. Hence, we expect that only a few votes are cached when a
// consumer is registered. For the purpose of forensics, we might register a
// consumer later, when already lots of votes are cached. However, this should
// be a rare occurrence (we except moderate performance overhead in this case).
func (vc *VotesCache) RegisterVoteConsumer(consumer VoteConsumer) {
	if consumer == nil {
		return
	}
	vc.lock.Lock()
	defer vc.lock.Unlock()
	vc.voteConsumers = append(vc.voteConsumers, consumer)

	// feed the consumer with the cached votes
	orderedVotes := make([]*model.Vote, len(vc.votes))
	for _, v := range vc.votes {
		orderedVotes[v.index] = v.Vote
	}
	for _, vote := range orderedVotes {
		consumer(vote) // non-blocking per API contract
	}
}

func (vc *VotesCache) ByBlockID(blockID flow.Identifier) []*model.Vote {
	vc.lock.Lock()
	defer vc.lock.Unlock()

	// First, put the cached votes in a slice in the order they were received.
	// But leave the elements as nil, where the vote does _not_ match `blockID`.
	matchingVotes := make([]*model.Vote, len(vc.votes))
	for _, v := range vc.votes {
		if v.Vote.BlockID == blockID {
			matchingVotes[v.index] = v.Vote
		}
	}

	// Second, consolidate all the non-nil elements in `matchingVotes` in the
	// head of the slice, while preserving the order.
	numberMatchingVotes := 0
	for i, v := range matchingVotes {
		if v == nil {
			continue
		}
		if numberMatchingVotes < i {
			matchingVotes[numberMatchingVotes] = v
		}
		numberMatchingVotes++
	}

	if numberMatchingVotes < 1 { // no vote for block found
		return nil
	}
	return matchingVotes[:numberMatchingVotes]
}
