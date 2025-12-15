package votecollector

import (
	"errors"
	"fmt"
	"sync"

	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/model/flow"
)

var (
	// RepeatedVoteErr is emitted, when we receive the _same_ vote for the same block
	// from the same voter multiple times. This error does _not_ indicate
	// equivocation.
	RepeatedVoteErr = errors.New("duplicated vote")
)

// voteContainer container stores the vote and an index representing
// the order in which the votes were received
type voteContainer struct {
	*model.Vote
	index int
}

// VotesCache maintains a _concurrency safe_ cache of votes for one particular
// view. The cache memorizes the order in which the votes were received. Votes
// are de-duplicated based on the following rules:
//
//  1. The cache only ingests votes for the pre-configured view. Votes
//     for other views are rejected with an [VoteForIncompatibleViewError].
//
//  2. For each voter (i.e. SignerID), we store the _first_ vote `v0`. For any subsequent
//     vote `v1` from the _same_ replica, we check:
//     2a. v1.ID() == v0.ID():
//     Votes with the same ID are identical. No protocol violation. We reject `v1` with a [RepeatedVoteErr]
//     2b. v1.ID() ≠ v0.ID():
//     The consensus replica has emitted inconsistent votes within the same view, which
//     is generally a protocol violation. This is a sign of vote equivocation which happen if
//     replica is voting for different blocks at the same view or using different signing information.
//     We reject `v1` with a [model.DoubleVoteError].
type VotesCache struct {
	// CAUTION: In the VoteCollector's liveness proof, we utilized that reading the `VotesCache` happens before writing to it. It is important to
	// emphasize that only locks are agnostic to the performed operation being a read or a write. In contrast, atomic variables only establish a
	// 'synchronized before' relation when a preceding write is observed by a subsequent read. However, the VoteProcessor first reads and then
	// writes. For atomic variables, this order of operations does not induce any synchronization guarantees according to Go Memory Model
	// ( https://go.dev/ref/mem ). Hence, the VotesCache utilizing locks is critical for the correctness of the `VoteCollector`.
	lock sync.RWMutex

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

// AddVote stores a vote in the cache. The method returns `nil`, if the vote was
// successfully stored. When AddVote returns an error, the vote is _not_ stored.
// Expected error returns during normal operations:
//   - [VoteForIncompatibleViewError] is returned if the vote is for a different view.
//   - [model.DoubleVoteError] indicates that the voter has emitted inconsistent
//     votes within the same view. We consider two votes as inconsistent, if they
//     are from the same signer for the same view, but have different IDs. Potential
//     causes could be:
//     (i) The signer voted for different blocks in the same view.
//     (ii) The signer emitted votes for the same block, but included different
//     signatures. This is only relevant for voting schemes where the signer has
//     different options how to sign (e.g. sign with staking key and/or random
//     beacon key). For such voting schemes, byzantine replicas could try to
//     submit different votes for the same block, to exhaust the vote collector's
//     resources or have multiple of their votes counted to undermine consensus
//     safety (aka double-counting attack).
//     Both (i) and (ii) are protocol violations and belong to the family of equivocation
//     attacks. As part of the error, we return the internally-cached vote, which is
//     conflicting with the input `vote`.
//   - [RepeatedVoteErr] is returned when adding a vote that is _identical_ (same ID)
//     to a previously added vote. This is not a slashable protocol violation.
func (vc *VotesCache) AddVote(vote *model.Vote) error {
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
			return RepeatedVoteErr
		}
		if firstVote.BlockID != vote.BlockID {
			// voting in the same view for different blocks => vote equivocation
			return model.NewDoubleVoteErrorf(firstVote.Vote, vote, "replica %v voted for different blocks in view %d", vote.SignerID, vc.view)
		}
		// Intentionally votes to not contain any auxiliary information that the voter can vary. Hence, the
		// sender is voting for the same block but supplying different signatures => vote equivocation
		return model.NewDoubleVoteErrorf(firstVote.Vote, vote, "detected vote equivocation at view: %d", vc.view)
	}

	// previously unknown vote: (1) store and (2) forward to consumers
	vc.votes[vote.SignerID] = voteContainer{vote, len(vc.votes)}
	for _, consumer := range vc.voteConsumers {
		consumer(vote)
	}
	return nil
}

// GetVote returns the stored vote for the given `signerID`. Returns:
//   - (vote, true) if a vote from signerID is known
//   - (false, nil) no vote from signerID is known
func (vc *VotesCache) GetVote(signerID flow.Identifier) (*model.Vote, bool) {
	vc.lock.RLock()
	container, exists := vc.votes[signerID] // if signerID is unknown, its `Vote` pointer is nil
	vc.lock.RUnlock()
	return container.Vote, exists
}

// Size returns the number of cached votes
func (vc *VotesCache) Size() int {
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
