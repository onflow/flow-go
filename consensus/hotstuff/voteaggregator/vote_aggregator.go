package voteaggregator

import (
	"fmt"

	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
)

// VoteAggregator stores the votes and aggregates them into a QC when enough votes have been collected
type VoteAggregator struct {
	notifier              hotstuff.Consumer
	committee             hotstuff.Committee
	voteValidator         hotstuff.Validator
	signer                hotstuff.SignerVerifier
	highestPrunedView     uint64
	pendingVotes          *PendingVotes                               // keeps track of votes whose blocks can not be found
	viewToBlockIDSet      map[uint64]map[flow.Identifier]struct{}     // for pruning
	viewToVoteID          map[uint64]map[flow.Identifier]*model.Vote  // for detecting double voting
	createdQC             map[flow.Identifier]*flow.QuorumCertificate // keeps track of QCs that have been made for blocks
	blockIDToVotingStatus map[flow.Identifier]*VotingStatus           // keeps track of accumulated votes and stakes for blocks
	proposerVotes         map[flow.Identifier]*model.Vote             // holds the votes of block proposers, so we can avoid passing around proposals everywhere
}

// New creates an instance of vote aggregator
func New(notifier hotstuff.Consumer, highestPrunedView uint64, committee hotstuff.Committee, voteValidator hotstuff.Validator, signer hotstuff.SignerVerifier) *VoteAggregator {
	return &VoteAggregator{
		notifier:              notifier,
		highestPrunedView:     highestPrunedView,
		committee:             committee,
		voteValidator:         voteValidator,
		signer:                signer,
		pendingVotes:          NewPendingVotes(),
		viewToBlockIDSet:      make(map[uint64]map[flow.Identifier]struct{}),
		viewToVoteID:          make(map[uint64]map[flow.Identifier]*model.Vote),
		createdQC:             make(map[flow.Identifier]*flow.QuorumCertificate),
		blockIDToVotingStatus: make(map[flow.Identifier]*VotingStatus),
		proposerVotes:         make(map[flow.Identifier]*model.Vote),
	}
}

// StorePendingVote stores the vote as a pending vote assuming the caller has checked that the voting
// block is currently missing.
// It's idempotent. Meaning, calling it again with the same block returns the same result.
// Note: Validations on these pending votes will be postponed until the block has been received.
func (va *VoteAggregator) StorePendingVote(vote *model.Vote) (bool, error) {
	// check if the vote is for a view that has already been pruned (and is thus stale)
	// cannot store vote for already pruned view
	if va.isVoteStale(vote) {
		return false, nil
	}

	// sanity check to see if block has been received or not
	_, exist := va.blockIDToVotingStatus[vote.BlockID]
	if exist {
		return false, fmt.Errorf("asked to store pending vote, but block has actually received: view: %v, vote ID: %v", vote.View, vote.ID())
	}

	// add vote, return false if the vote is not successfully added (already existed)
	ok := va.pendingVotes.AddVote(vote)
	if !ok {
		return false, nil
	}
	va.updateState(vote)
	return true, nil
}

// StoreVoteAndBuildQC stores the vote assuming the caller has checked that the voting block is incorporated,
// and returns a QC if there are votes with enough stakes.
// It's idempotent. Meaning, calling it again with the same block returns the same result.
// The VoteAggregator builds a QC as soon as the number of votes allow this.
// While subsequent votes (past the required threshold) are not included in the QC anymore,
// VoteAggregator ALWAYS returns the same QC as the one returned before.
func (va *VoteAggregator) StoreVoteAndBuildQC(vote *model.Vote, block *model.Block) (*flow.QuorumCertificate, bool, error) {
	if vote.BlockID != block.BlockID {
		return nil, false, fmt.Errorf("vote and block don't have the same block ID: vote.BlockID (%v), block.BlockID: (%v)",
			vote.BlockID, block.BlockID)
	}

	// if the QC for the block has been created before, return the QC
	oldQC, built := va.createdQC[block.BlockID]
	if built {
		return oldQC, true, nil
	}

	// ignore stale votes
	if va.isVoteStale(vote) {
		return nil, false, nil
	}

	// It's possible we receive votes before the block, in that case when we just receive the block, we should
	// convert all pending votes and check if there are enough votes to build a QC.
	// And when building QC with votes, we would like to give priority to our own vote and proposer vote to be
	// included in the QC.
	shouldConvertVotes := !va.isBlockReceived(block)

	// validate the vote and adding it to the accumulated voting status
	// if `shouldConvertVotes` is true, meaning we just received the block, then EventHandler has ensured
	// the vote is actually our own vote, in this case, we will give our own vote priority to be stored first.
	valid, err := va.validateAndStoreIncorporatedVote(vote, block)
	if err != nil {
		return nil, false, fmt.Errorf("could not store incorporated vote: %w", err)
	}

	// cannot build qc if vote is invalid
	if !valid {
		return nil, false, nil
	}

	// if we haven't received the block yet, then should call BuildQCOnReceivedBlock first
	// to convert all pending votes, including the proposer's vote.
	if shouldConvertVotes {
		newQC, built, err := va.BuildQCOnReceivedBlock(block)
		if err != nil {
			return nil, false, fmt.Errorf("can not build qc on receive block: %w", err)
		}

		if built {
			return newQC, true, nil
		}

		// if we can't build a QC, then we can just return, because we have stored the vote already
		return nil, false, nil
	}

	// try to build the QC with existing votes
	newQC, built, err := va.tryBuildQC(block.BlockID)
	if err != nil {
		return nil, false, fmt.Errorf("could not build QC: %w", err)
	}

	return newQC, built, nil
}

func (va *VoteAggregator) isBlockReceived(block *model.Block) bool {
	// We won't create voting status until the block is received. Therefore, if voting status
	// has been created for a certain block, the block must have been received.
	_, exist := va.blockIDToVotingStatus[block.BlockID]
	return exist
}

// StoreProposerVote stores the vote for a block that was proposed.
func (va *VoteAggregator) StoreProposerVote(vote *model.Vote) bool {
	// check if the proposer vote is for a view that has already been pruned (and is thus stale)
	if va.isVoteStale(vote) { // cannot store vote for already pruned view
		return false
	}
	// add proposer vote, return false if it exists
	_, exists := va.proposerVotes[vote.BlockID]
	if exists {
		return false
	}
	va.proposerVotes[vote.BlockID] = vote
	// update viewToBlockIDSet
	blockIDSet, exists := va.viewToBlockIDSet[vote.View]
	if exists {
		blockIDSet[vote.BlockID] = struct{}{}
	} else {
		blockIDSet = make(map[flow.Identifier]struct{})
		blockIDSet[vote.BlockID] = struct{}{}
		va.viewToBlockIDSet[vote.View] = blockIDSet
	}
	return true
}

// BuildQCOnReceivedBlock will attempt to build a QC for the given block when there are votes
// with enough stakes.
// It's idempotent. Meaning, calling it again with the same block returns the same result.
// It assumes that the proposer's vote has been stored by calling StoreProposerVote
// It assumes the block has been validated.
// It returns (qc, true, nil) if a QC is built
// It returns (nil, false, nil) if not enough votes to build a QC.
// It returns (nil, false, nil) if the block is stale
// It returns (nil, false, err) if there is an unknown error
func (va *VoteAggregator) BuildQCOnReceivedBlock(block *model.Block) (*flow.QuorumCertificate, bool, error) {
	// return the QC that was built before if exists
	oldQC, exists := va.createdQC[block.BlockID]
	if exists {
		return oldQC, true, nil
	}

	// if the block is stale, we just return nil
	if va.isBlockStale(block) {
		return nil, false, nil
	}

	// proposer vote is the first to be accumulated
	proposerVote, exists := va.proposerVotes[block.BlockID]
	if !exists {
		// proposer must has been stored before, otherwise it's a bug.
		// cannot build qc if proposer vote does not exist
		return nil, false, fmt.Errorf("could not get proposer vote for block: %x, blockView: %v, highestPrunedView: %v", block.BlockID, block.View, va.highestPrunedView)
	}

	// accumulate proposer's vote first to ensure proposer's vote is always included in the QC.
	// a consensus node is rewarded by having proposed block being included in the finalized chain.
	// Including the proposer's vote first guarantees that the proposer will get rewarded if the block
	// is finalized. It incentivizes an honest leader to propose block.
	valid, err := va.validateAndStoreIncorporatedVote(proposerVote, block)
	if err != nil {
		return nil, false, fmt.Errorf("could not validate proposer vote: %w", err)
	}
	if !valid {
		// when the proposer vote is invalid, it means the assumption of the block
		// being valid is broken. In this case, throw an error
		return nil, false, fmt.Errorf("validating block failed: %x", block.BlockID)
	}

	// accumulate pending votes by order
	pendingStatus, exists := va.pendingVotes.votes[block.BlockID]
	if exists {
		err = va.convertPendingVotes(pendingStatus.orderedVotes, block)
		if err != nil {
			return nil, false, fmt.Errorf("could not build QC on receiving block: %w", err)
		}
	}

	// try building QC with existing valid votes
	qc, built, err := va.tryBuildQC(block.BlockID)
	if err != nil {
		return nil, false, fmt.Errorf("could not build QC on receiving block: %w", err)
	}
	return qc, built, nil
}

// PruneByView will delete all votes equal or below to the given view, as well as related indexes.
func (va *VoteAggregator) PruneByView(view uint64) {
	if view <= va.highestPrunedView {
		return
	}
	for i := va.highestPrunedView + 1; i <= view; i++ {
		blockIDStrSet := va.viewToBlockIDSet[i]
		for blockID := range blockIDStrSet {
			delete(va.pendingVotes.votes, blockID)
			delete(va.blockIDToVotingStatus, blockID)
			delete(va.createdQC, blockID)
			delete(va.proposerVotes, blockID)
		}
		delete(va.viewToBlockIDSet, i)
		delete(va.viewToVoteID, i)
	}
	va.highestPrunedView = view
}

// convertPendingVotes goes over the pending votes one by one and adds them to the block's VotingStatsus
// until enough votes are accumulated. It guarantees that only the minimal number of votes are added.
func (va *VoteAggregator) convertPendingVotes(pendingVotes []*model.Vote, block *model.Block) error {
	for _, vote := range pendingVotes {
		// if threshold is reached, BEFORE adding the vote, vote and all subsequent votes can be ignored
		if va.canBuildQC(block.BlockID) {
			break
		}
		// otherwise, validate and add vote
		valid, err := va.validateAndStoreIncorporatedVote(vote, block)
		if !valid {
			continue
		}
		if err != nil {
			return fmt.Errorf("processing pending votes failed: %w", err)
		}
	}
	delete(va.pendingVotes.votes, block.BlockID)
	return nil
}

// storeIncorporatedVote stores incorporated votes and accumulates weight
// It drops invalid votes.
//
// Handling of DOUBLE VOTES (equivocation):
// Including double votes in building the QC for their respective block does _not_
// reduce the security. The main deterrent is the slashing for equivocation, which only
// requires detection of double voting. Therefore, we take the simpler approach and do
// not discard votes from equivocating nodes. When encountering vote equivocation:
//   * notify consumer
//   * do not error, as a replica must handle this case is part of the "normal operation"
//   * Provided the vote is valid by itself, treat it as valid. The validity of a vote
//     needs to be objective and not depend on whether the replica has knowledge of a
//     conflicting vote.
//
// Note that treating a valid vote as invalid would create the following additional edge case:
//   * consider a primary that is proposing two conflicting blocks (block equivocation)
//   * Assume the case where we would treat the proposer's vote for its second block
//     (embedded in the block) as invalid
//   * then, we would arrive at the conclusion that the second block itself is invalid
// This would violate objective validity of blocks.
func (va *VoteAggregator) validateAndStoreIncorporatedVote(vote *model.Vote, block *model.Block) (bool, error) {
	// validate the vote
	voter, err := va.voteValidator.ValidateVote(vote, block)
	if model.IsInvalidVoteError(err) {
		// does not report invalid vote as an error, notify consumers instead
		va.notifier.OnInvalidVoteDetected(vote)
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("could not validate incorporated vote: %w", err)
	}

	// check for double vote:
	firstVote, detected := va.detectDoubleVote(vote)
	if detected {
		va.notifier.OnDoubleVotingDetected(firstVote, vote)
	}

	// update existing voting status or create a new one
	votingStatus, exists := va.blockIDToVotingStatus[vote.BlockID]
	if !exists {
		// get all identities
		identities, err := va.committee.Identities(vote.BlockID, filter.Any)
		if err != nil {
			return false, fmt.Errorf("error retrieving consensus participants: %w", err)
		}

		// create VotingStatus for block
		stakeThreshold := hotstuff.ComputeStakeThresholdForBuildingQC(identities.TotalStake()) // stake threshold for building valid qc
		votingStatus = NewVotingStatus(block, stakeThreshold, va.signer)
		va.blockIDToVotingStatus[vote.BlockID] = votingStatus
	}
	votingStatus.AddVote(vote, voter)
	va.updateState(vote)
	return true, nil
}

func (va *VoteAggregator) updateState(vote *model.Vote) {
	voterID := vote.SignerID

	// update viewToVoteID
	idToVote, exists := va.viewToVoteID[vote.View]
	if exists {
		idToVote[voterID] = vote
	} else {
		idToVote = make(map[flow.Identifier]*model.Vote)
		idToVote[voterID] = vote
		va.viewToVoteID[vote.View] = idToVote
	}

	// update viewToBlockIDSet
	blockIDSet, exists := va.viewToBlockIDSet[vote.View]
	if exists {
		blockIDSet[vote.BlockID] = struct{}{}
	} else {
		blockIDSet = make(map[flow.Identifier]struct{})
		blockIDSet[vote.BlockID] = struct{}{}
		va.viewToBlockIDSet[vote.View] = blockIDSet
	}
}

func (va *VoteAggregator) tryBuildQC(blockID flow.Identifier) (*flow.QuorumCertificate, bool, error) {
	votingStatus, exists := va.blockIDToVotingStatus[blockID]
	if !exists { // can not build a qc if voting status doesn't exist
		return nil, false, nil
	}
	qc, built, err := votingStatus.TryBuildQC()
	if err != nil { // can not build a qc if there is an error
		return nil, false, err
	}
	if !built { // votes are insufficient
		return nil, false, nil
	}

	va.createdQC[blockID] = qc
	return qc, true, nil
}

// double voting is detected when the voter has voted a different block at the same view before
func (va *VoteAggregator) detectDoubleVote(vote *model.Vote) (*model.Vote, bool) {
	idToVotes, ok := va.viewToVoteID[vote.View]
	if !ok {
		// never voted by anyone
		return nil, false
	}
	originalVote, exists := idToVotes[vote.SignerID]
	if !exists {
		// never voted by this sender
		return nil, false
	}
	if originalVote.BlockID == vote.BlockID {
		// voted and is the same vote as the vote received before
		return nil, false
	}
	return originalVote, true
}

func (va *VoteAggregator) canBuildQC(blockID flow.Identifier) bool {
	votingStatus, exists := va.blockIDToVotingStatus[blockID]
	if !exists {
		return false
	}
	return votingStatus.CanBuildQC()
}

func (va *VoteAggregator) isVoteStale(vote *model.Vote) bool {
	return vote.View <= va.highestPrunedView
}

func (va *VoteAggregator) isBlockStale(block *model.Block) bool {
	return block.View <= va.highestPrunedView
}
