package votecollector

import (
	"errors"
	"sync"

	"github.com/onflow/flow-go/consensus/hotstuff"
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

// VotesCache maintains a _concurrency safe_ cache of votes for one particular
// view. The cache memorizes the order in which the votes were received. Votes
// are de-duplicated based on the following rules:
//  * Vor each voter (i.e. SignerID), we store the _first_ vote v0.
//  * For any subsequent vote v, we check whether v.BlockID == v0.BlockID.
//    If this is the case, we consider the vote a duplicate and drop it.
//    If v and v0 have different BlockIDs, the voter is equivocating and
//    we return a model.DoubleVoteError
// .
type VotesCache struct {
	lock          sync.Mutex
	view          uint64
	votes         map[flow.Identifier]voteContainer // signerID -> first vote
	voteConsumers []hotstuff.VoteConsumer
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
//    (i.e. voting in the same view for different blocks).
//  * RepeatedVoteErr is returned when adding a vote for the same block from
//    the same voter multiple times.
//  * IncompatibleViewErr is returned if the vote is for a different view.
// When AddVote returns an error, the vote is _not_ stored.
func (vc *VotesCache) AddVote(vote *model.Vote) error {
	if vote.View != vc.view {
		return VoteForIncompatibleViewError
	}
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
func (vc *VotesCache) RegisterVoteConsumer(consumer hotstuff.VoteConsumer) {
	vc.lock.Lock()
	defer vc.lock.Unlock()

	vc.voteConsumers = append(vc.voteConsumers, consumer)
	for _, vote := range vc.all() { // feed the consumer with the cached votes
		consumer(vote) // non-blocking per API contract
	}
}

// All returns all currently cached votes. Concurrency safe.
func (vc *VotesCache) All() []*model.Vote {
	vc.lock.Lock()
	defer vc.lock.Unlock()
	return vc.all()
}

// all returns all currently cached votes. NOT concurrency safe
func (vc *VotesCache) all() []*model.Vote {
	orderedVotes := make([]*model.Vote, len(vc.votes))
	for _, v := range vc.votes {
		orderedVotes[v.index] = v.Vote
	}
	return orderedVotes
}
