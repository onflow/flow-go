package votecollector

import (
	"fmt"
	"sync"

	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/model/flow"
)

// voteContainer container stores the vote and in index representing
// the order in which the votes were received
type voteContainer struct {
	*model.Vote
	index int
}

// ViewSpecificVotesCache maintains a _concurrency safe_ cache of votes for one particular
// view. The cache memorizes the order in which the votes were received. Votes are rejected
// or de-duplicated based on the following rules:
//  1. The cache only ingests votes for the pre-configured view. Votes
//     for other views are rejected with an `VoteForIncompatibleViewError`.
//  2. For each voter (i.e. SignerID), we store the _first_ vote `v0`. For any subsequent
//     vote `v1` from the _same_ replica, we check:
//     2a: v1.ID() == v0.ID()
//         Votes with the same ID are identical. No protocol violation.
//         We reject `v1` with a `DuplicatedVoteErr`
//     2b: v1.ID() ≠ v0.ID()
//         The consensus replica has emitted inconsistent votes within the same view, which
//         is generally a protocol violation. We reject `v1` with an `InconsistentVoteError`.
type ViewSpecificVotesCache struct {
	lock          sync.RWMutex
	view          uint64
	votes         map[flow.Identifier]voteContainer // signerID -> first vote
	voteConsumers []hotstuff.VoteConsumer
}

// NewViewSpecificVotesCache instantiates a ViewSpecificVotesCache for the given view
func NewViewSpecificVotesCache(view uint64) *ViewSpecificVotesCache {
	return &ViewSpecificVotesCache{
		view:  view,
		votes: make(map[flow.Identifier]voteContainer),
	}
}

func (vc *ViewSpecificVotesCache) View() uint64 { return vc.view }

// AddVote stores a vote in the cache. The method returns `nil`, if the vote was
// successfully stored. When AddVote returns an error, the vote is _not_ stored.
// Expected error returns during normal operations:
//  * VoteForIncompatibleViewError is returned if the vote is for a different view.
//  * model.InconsistentVoteError indicates that the voter has emitted inconsistent
//    votes within the same view. We consider two votes as inconsistent, if they
//    are from the same signer for the same view, but have different IDs. Potential
//    causes could be:
//    - The signer voted for different blocks in the same view.
//    - The signer emitted votes for the same block, but included different
//      signatures. This is only relevant for voting schemes where the signer has
//      different options how to sign (e.g. sign with staking key and/or random
//      beacon key). For such voting schemes, byzantine replicas could try to
//      submit different votes for the same block, to exhaust the primary's
//      resources or have multiple of their votes counted to undermine consensus
//      safety (aka double-counting attack).
//    Both are protocol violations and belong to the family of equivocation attacks.
//    As part of the error, we return the internally-cached vote, which is conflicting
//    with the input `vote`.
//  * DuplicatedVoteErr is returned when adding a vote that is _identical_ (same ID)
//    to a previously added vote.
func (vc *ViewSpecificVotesCache) AddVote(vote *model.Vote) error {
	if vote.View != vc.view {
		return fmt.Errorf("expected vote for view %d but vote's view is %d: %w", vc.view, vote.View, VoteForIncompatibleViewError)
	}
	vc.lock.Lock()
	defer vc.lock.Unlock()

	// ensure that this is the first vote from this SignerID
	firstVote, exists := vc.votes[vote.SignerID]
	if exists {
		if firstVote.ID() != vote.ID() {
			return model.NewInconsistentVoteErrorf(firstVote.Vote, "in view %d, replica %v emitted inconsistent votes %v and %v", vc.view, vote.SignerID, firstVote.ID(), vote.ID())
		}
		return DuplicatedVoteErr // votes are identical
	}

	// store vote forward to VoteConsumers
	vc.votes[vote.SignerID] = voteContainer{vote, len(vc.votes)}
	for _, consumer := range vc.voteConsumers {
		consumer(vote)
	}
	return nil
}

// GetVote returns the stored vote for the given `signerID`. Returns:
//  - (vote, true) if a vote from signerID is known
//  - (nil, false) no vote from signerID is known
func (vc *ViewSpecificVotesCache) GetVote(signerID flow.Identifier) (*model.Vote, bool) {
	vc.lock.RLock()
	container, exists := vc.votes[signerID] // if signerID is unknown, its `Vote` pointer is nil
	vc.lock.RUnlock()
	return container.Vote, exists
}

// Size returns the number of cached votes
func (vc *ViewSpecificVotesCache) Size() int {
	vc.lock.RLock()
	s := len(vc.votes)
	vc.lock.RUnlock()
	return s
}

// RegisterVoteConsumer registers a VoteConsumer. Upon registration, the cache
// feeds all cached votes into the consumer in the order they arrived.
// CAUTION: a consumer _must_ be non-blocking and consume the votes without
// noteworthy delay. Otherwise, consensus speed is impacted.
//
// Expected usage pattern: During happy-path operations, the block arrives in a
// timely manner. Hence, we expect that only a few votes are cached when a
// consumer is registered. For the purpose of forensics, we might register a
// consumer later, when already lots of votes are cached. However, this should
// be a rare occurrence (we except moderate performance overhead in this case).
func (vc *ViewSpecificVotesCache) RegisterVoteConsumer(consumer hotstuff.VoteConsumer) {
	vc.lock.Lock()
	defer vc.lock.Unlock()

	vc.voteConsumers = append(vc.voteConsumers, consumer)
	for _, vote := range vc.all() { // feed the consumer with the cached votes
		consumer(vote) // non-blocking per API contract
	}
}

// All returns all currently cached votes in the order they were received. Concurrency safe.
func (vc *ViewSpecificVotesCache) All() []*model.Vote {
	vc.lock.RLock()
	defer vc.lock.RUnlock()
	return vc.all()
}

// all returns all currently cached votes in the order they were received. NOT concurrency safe.
func (vc *ViewSpecificVotesCache) all() []*model.Vote {
	orderedVotes := make([]*model.Vote, len(vc.votes))
	for _, v := range vc.votes {
		orderedVotes[v.index] = v.Vote
	}
	return orderedVotes
}

// BlockSpecificVotesCache maintains a _concurrency safe_ cache of votes for one particular
// block. It accepts votes if view and block ID match. The cache memorizes the order in which
// the votes were received. Votes are rejected or de-duplicated based on the following rules:
//  1. The cache only ingests votes for the pre-configured blockID. Votes
//     with other BlockIDs are rejected with an `VoteForIncompatibleBlockError`.
//  2. As a sanity check, we check that the vote's view matches the view of the pre-configured
//     block. Votes for other views are rejected with an `VoteForIncompatibleViewError`.
//  3. For each voter (i.e. SignerID), we store the _first_ vote `v0`. For any subsequent
//     vote `v1` from the _same_ replica, we check:
//     2a: v1.ID() == v0.ID()
//         Votes with the same ID are identical. No protocol violation.
//         We reject `v1` with a `DuplicatedVoteErr`
//     2b: v1.ID() ≠ v0.ID()
//         The consensus replica has emitted inconsistent votes within the same view, which
//         is generally a protocol violation. We reject `v1` with an `InconsistentVoteError`.
//
// Comment: In comparison to `ViewSpecificVotesCache`, this cache implementation cannot detect
// double-voting attacks (voting for different blocks in the same view). This is because the
// cache only accepts votes for a pre-configured BlockID and rejects votes for different
// blocks with VoteForIncompatibleViewError
type BlockSpecificVotesCache struct {
	blockID flow.Identifier
	ViewSpecificVotesCache
}

// NewBlockSpecificVotesCache instantiates a VotesCache that only accepts votes
// for a specific block, where we expect votes to match view and block ID.
func NewBlockSpecificVotesCache(block *model.Block) *BlockSpecificVotesCache {
	return &BlockSpecificVotesCache{
		blockID:                block.BlockID,
		ViewSpecificVotesCache: *NewViewSpecificVotesCache(block.View),
	}
}

// BlockID returns the block ID that votes must have in order to be accepted by the cache.
func (vc *BlockSpecificVotesCache) BlockID() flow.Identifier { return vc.blockID }

// AddVote stores a vote in the cache. The method returns `nil`, if the vote was
// successfully stored. When AddVote returns an error, the vote is _not_ stored.
// Expected error returns during normal operations:
//  * nil: if the vote was successfully added
//  * VoteForIncompatibleBlockError is returned if the vote is for a different block ID.
//  * VoteForIncompatibleViewError is returned if the vote is for a different view.
//  * model.DEP_InconsistentVoteError indicates that the voter has emitted inconsistent
//    votes for the _same_ block but with inconsistent signatures.
//    Comment: This cache rejects all votes that are not for the expected block ID
//    with a `VoteForIncompatibleBlockError`. Therefore, it doesn't detect
//    double-voting attacks (voting for different blocks in the same view).
//  * DuplicatedVoteErr is returned when adding a vote that is _identical_
//    to a previously added vote.
func (vc *BlockSpecificVotesCache) AddVote(vote *model.Vote) error {
	if vote.BlockID != vc.blockID {
		return fmt.Errorf("expecting votes for block %v, but got vote for block %v: %w", vc.blockID, vote.BlockID, VoteForIncompatibleBlockError)
	}
	return vc.ViewSpecificVotesCache.AddVote(vote)
}
