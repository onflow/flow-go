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
// view. The cache memorizes the order in which the votes were received. Votes
// are rejected or de-duplicated based on the following rules:
//  1. The votes cache only ingests votes for the pre-configured view.
//     Votes for other views are rejected with an `VoteForIncompatibleViewError`.
//  2. For each voter (i.e. SignerID), we store the _first_ vote `v0`. For any subsequent
//     vote `v1` from the _same_ replica, we check:
//     2a: v1.ID() == v0.ID()
//         Votes with the same ID are identical. No protocol violation.
//         We reject `v1` with a `DuplicatedVoteErr`
//     2b: v1.BlockID == v0.BlockID _but_ v1.ID() ≠ v0.ID()
//         The replica voted for the same block but supplied different signatures.
//         Depending on the signature scheme, this _might_ be a symptom of an attack
//         where the voter attempts to have their vote repeatedly counted.
//         We reject the vote with a `InconsistentVoteError`.
//     2c: v1.BlockID ≠ v0.BlockID
//         The replica voted for different blocks in the same view, i.e. it is
//         equivocating, which is a severe protocol violation. We reject the vote
//         with a `DoubleVoteError`
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

// AddVote stores a vote in the cache. The following errors are expected during
// normal operations:
//  * nil: if the vote was successfully added
//  * VoteForIncompatibleViewError is returned if the vote is for a different view.
//  * model.DoubleVoteError is returned if the voter is equivocating,
//    i.e. voting in the same view for _different_ blocks.
//  * model.InconsistentVoteError is returned if the voter emitted
//    votes for the _same_ block but with inconsistent signatures.
//  * DuplicatedVoteErr is returned when adding a vote that is _identical_
//    to a previously added vote.
// When AddVote returns an error, the vote is _not_ stored.
func (vc *ViewSpecificVotesCache) AddVote(vote *model.Vote) error {
	if vote.View != vc.view {
		return fmt.Errorf("expected vote for view %d but vote's view is %d: %w", vc.view, vote.View, VoteForIncompatibleViewError)
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
		if firstVote.ID() == vote.ID() {
			// votes are identical
			return DuplicatedVoteErr
		}
		if firstVote.BlockID != vote.BlockID {
			// voting in the same view for different blocks => double vote attack
			return model.NewDoubleVoteErrorf(firstVote.Vote, vote, "in view %d, replica %v voted for different blocks %v and %v", vc.view, vote.SignerID, firstVote.BlockID, vote.BlockID)
		}
		// voting for the same block but supplying different signatures => inconsistent vote attack
		return model.NewInconsistentVoteErrorf(firstVote.Vote, vote, "in view %d, replica %v emitted inconsistent votes %v and %v for block %v: %d", vc.view, vote.SignerID, firstVote.ID(), vote.ID(), firstVote.BlockID)
	}

	// previously unknown vote: (1) store and (2) forward to consumers
	vc.votes[vote.SignerID] = voteContainer{vote, len(vc.votes)}
	for _, consumer := range vc.voteConsumers {
		consumer(vote)
	}
	return nil
}

// GetVote returns the stored vote for the given `signerID`. Returns:
//  - (vote, true) if a vote from signerID is known
//  - (false, nil) no vote from signerID is known
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
// Expected usage patter: During happy-path operations, the block arrives in a
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

// All returns all currently cached votes. Concurrency safe.
func (vc *ViewSpecificVotesCache) All() []*model.Vote {
	vc.lock.Lock()
	defer vc.lock.Unlock()
	return vc.all()
}

// all returns all currently cached votes. NOT concurrency safe
func (vc *ViewSpecificVotesCache) all() []*model.Vote {
	orderedVotes := make([]*model.Vote, len(vc.votes))
	for _, v := range vc.votes {
		orderedVotes[v.index] = v.Vote
	}
	return orderedVotes
}

// BlockSpecificVotesCache accepts only votes for a specific block, where we
// expect the vote to match view and block ID.
// BlockSpecificVotesCache maintains a _concurrency safe_ cache of votes for one particular
// block. The cache memorizes the order in which the votes were received. Votes
// are rejected or de-duplicated based on the following rules:
//  1. The votes cache only ingests votes for the pre-configured blockID.
//     Votes with other BlockIDs are rejected with an `VoteForIncompatibleBlockError`.
//  2. As a sanity check, we check that the vote's view matches the view of the pre-configured
//     block. Votes for other views are rejected with an `VoteForIncompatibleViewError`.
//  3. For each voter (i.e. SignerID), we store the _first_ vote `v0`. For any subsequent
//     vote `v1` from the _same_ replica, we check:
//     2a: v1.ID() == v0.ID()
//         Votes with the same ID are identical. No protocol violation.
//         We reject `v1` with a `DuplicatedVoteErr`
//     2b: v1.BlockID == v0.BlockID _but_ v1.ID() ≠ v0.ID()
//         The replica voted for the same block but supplied different signatures.
//         Depending on the signature scheme, this _might_ be a symptom of an attack
//         where the voter attempts to have their vote repeatedly counted.
//         We reject the vote with a `InconsistentVoteError`.
//
// Comment: In comparison to `ViewSpecificVotesCache`, this cache does not
// detect double-voting attacks (voting for different blocks in the same view).
// This is because this cache only accepts votes for a pre-configured BlockID
// and rejects other votes with an `VoteForIncompatibleBlockError`.
type BlockSpecificVotesCache struct {
	blockID flow.Identifier
	ViewSpecificVotesCache
}

// NewBlockSpecificVotesCache instantiates a VotesCache that only accepts votes
// for a specific block, where we expect the vote to match view and block ID.
func NewBlockSpecificVotesCache(block *model.Block) *BlockSpecificVotesCache {
	return &BlockSpecificVotesCache{
		blockID:                block.BlockID,
		ViewSpecificVotesCache: *NewViewSpecificVotesCache(block.View),
	}
}

// BlockID returns the block ID that votes must have in order to be accepted by the cache.
func (vc *BlockSpecificVotesCache) BlockID() flow.Identifier { return vc.blockID }

// AddVote stores a vote in the cache. The following errors are expected during
// normal operations:
//  * nil: if the vote was successfully added
//  * VoteForIncompatibleBlockError is returned if the vote is for a different block ID.
//  * VoteForIncompatibleViewError is returned if the vote is for a different view.
//  * model.InconsistentVoteError is returned if the voter emitted
//    votes for the _same_ block but with inconsistent signatures
//  * DuplicatedVoteErr is returned when adding a vote that is _identical_
//    to a previously added vote.
// When AddVote returns an error, the vote is _not_ stored.
//
// Comment: This cache rejects all votes that are not for the expected block ID
// with an `VoteForIncompatibleBlockError`. Therefore, it doesn't detect
// double-voting attacks (voting for different blocks in the same view).
func (vc *BlockSpecificVotesCache) AddVote(vote *model.Vote) error {
	if vote.BlockID != vc.blockID {
		return fmt.Errorf("expecting votes for block %v, but got vote for block %v: %w ", vc.blockID, vote.BlockID, VoteForIncompatibleBlockError)
	}
	return vc.ViewSpecificVotesCache.AddVote(vote)
}
