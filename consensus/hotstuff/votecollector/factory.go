package votecollector

import (
	"fmt"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/consensus/hotstuff/signature"
)

// baseFactory instantiates VerifyingVoteProcessors. Depending on the specific signing
// scheme (e.g. for main consensus, or collector clusters), a different baseFactory can
// be used.
// CAUTION: the baseFactory creates the VerifyingVoteProcessor for the given block. It
// does _not_ check the proposer's vote for its own block. The API reflects this by
// expecting a `model.Block` as input (which does _not_ contain the proposer vote) as
// opposed to `model.Proposal` (combines block with proposer's vote).
// Therefore, baseFactory does _not_ implement `hotstuff.VoteProcessorFactory` by itself.
// The VoteProcessorFactory adds the missing logic to verify the proposer's vote, by
// wrapping the baseFactory (decorator pattern).
type baseFactory func(log zerolog.Logger, block *model.Block) (hotstuff.VerifyingVoteProcessor, error)

// VoteProcessorFactory implements `hotstuff.VoteProcessorFactory`. Its main purpose
// is to construct instances of VerifyingVoteProcessors for a given block proposal.
// VoteProcessorFactory
// * delegates the creation of the actual instances to baseFactory
// * adds the logic to verify the proposer's vote for its own block
// Thereby, VoteProcessorFactory guarantees that only proposals with valid proposer
// vote are accepted (as per API specification). Otherwise, an `model.InvalidBlockError`
// is returned.
type VoteProcessorFactory struct {
	baseFactory baseFactory
}

var _ hotstuff.VoteProcessorFactory = (*VoteProcessorFactory)(nil)

// Create instantiates a VerifyingVoteProcessor for the given block proposal.
// A VerifyingVoteProcessor are only created for proposals with valid proposer votes.
// Expected error returns during normal operations:
// * model.InvalidBlockError - proposal has invalid proposer vote
func (f *VoteProcessorFactory) Create(log zerolog.Logger, proposal *model.Proposal) (hotstuff.VerifyingVoteProcessor, error) {
	processor, err := f.baseFactory(log, proposal.Block)
	if err != nil {
		return nil, fmt.Errorf("instantiating vote processor for block %v failed: %w", proposal.Block.BlockID, err)
	}

	err = processor.Process(proposal.ProposerVote())
	if err != nil {
		if model.IsInvalidVoteError(err) {
			return nil, model.InvalidBlockError{
				BlockID: proposal.Block.BlockID,
				View:    proposal.Block.View,
				Err:     fmt.Errorf("invalid proposer vote: %w", err),
			}
		}
		return nil, fmt.Errorf("processing proposer's vote for block %v failed: %w", proposal.Block.BlockID, err)
	}
	return processor, nil
}

// NewStakingVoteProcessorFactory implements hotstuff.VoteProcessorFactory for
// members of a collector cluster. For their cluster-local hotstuff, collectors
// only sign with their staking key.
func NewStakingVoteProcessorFactory(committee hotstuff.Committee, onQCCreated hotstuff.OnQCCreated) *VoteProcessorFactory {
	base := &stakingVoteProcessorFactoryBase{
		committee:   committee,
		onQCCreated: onQCCreated,
	}
	return &VoteProcessorFactory{
		baseFactory: base.Create,
	}
}

// NewCombinedVoteProcessorFactory implements hotstuff.VoteProcessorFactory fo
// participants of the Main Consensus Committee.
//
// With their vote, members of the main consensus committee can contribute to hotstuff and
// the random beacon. When a consensus participant signs with its random beacon key, it
// contributes to HotStuff consensus _and_ the Random Beacon. As a fallback, a consensus
// participant can sign with its staking key; thereby it contributes only to consensus but
// not the random beacon. There should be an economic incentive for the nodes to preferably
// sign with their random beacon key.
func NewCombinedVoteProcessorFactory(committee hotstuff.Committee, onQCCreated hotstuff.OnQCCreated) *VoteProcessorFactory {
	base := &combinedVoteProcessorFactoryBaseV2{
		committee:   committee,
		onQCCreated: onQCCreated,
		packer:      signature.NewConsensusSigDataPacker(committee),
	}
	return &VoteProcessorFactory{
		baseFactory: base.Create,
	}
}

/* ***************************** VerifyingVoteProcessor constructors for bootstrapping ***************************** */

// NewBootstrapCombinedVoteProcessor directly creates a CombinedVoteProcessorV2,
// suitable for the collector's local cluster consensus.
// Intended use: only for bootstrapping.
// UNSAFE: the proposer vote for `block` is _not_ validated or included
func NewBootstrapCombinedVoteProcessor(log zerolog.Logger, committee hotstuff.Committee, block *model.Block, onQCCreated hotstuff.OnQCCreated) (hotstuff.VerifyingVoteProcessor, error) {
	factory := &combinedVoteProcessorFactoryBaseV2{
		committee:   committee,
		onQCCreated: onQCCreated,
		packer:      signature.NewConsensusSigDataPacker(committee),
	}
	return factory.Create(log, block)
}

// NewBootstrapStakingVoteProcessor directly creates a `StakingVoteProcessor`,
// suitable for the collector's local cluster consensus.
// Intended use: only for bootstrapping.
// UNSAFE: the proposer vote for `block` is _not_ validated or included
func NewBootstrapStakingVoteProcessor(log zerolog.Logger, committee hotstuff.Committee, block *model.Block, onQCCreated hotstuff.OnQCCreated) (hotstuff.VerifyingVoteProcessor, error) {
	factory := &stakingVoteProcessorFactoryBase{
		committee:   committee,
		onQCCreated: onQCCreated,
	}
	return factory.Create(log, block)
}
