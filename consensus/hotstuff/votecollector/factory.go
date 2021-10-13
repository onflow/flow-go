package votecollector

import (
	"fmt"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/consensus/hotstuff/signature"
	"github.com/onflow/flow-go/consensus/hotstuff/verification"
	"github.com/onflow/flow-go/model/encoding"
	"github.com/onflow/flow-go/model/flow/filter"
)

// VoteProcessorFactory is a proxy class that delegates construction of vote processor to
// base factory. Base factory is responsible only for creating objects of combined or staking
// vote processors while VoteProcessorFactory is responsible for processing proposer vote and
// returning constructed object.
type VoteProcessorFactory struct {
	base hotstuff.VoteProcessorFactory
}

// CombinedBaseVoteProcessorFactory implements a factory for creating CombinedVoteProcessor
// holds needed dependencies to initialize CombinedVoteProcessor.
type CombinedBaseVoteProcessorFactory struct {
	log         zerolog.Logger
	packer      hotstuff.Packer
	committee   hotstuff.Committee
	onQCCreated hotstuff.OnQCCreated
}

var _ hotstuff.VoteProcessorFactory = &CombinedBaseVoteProcessorFactory{}
var _ hotstuff.VoteProcessorFactory = &VoteProcessorFactory{}

func NewCombinedVoteProcessorFactory(log zerolog.Logger, committee hotstuff.Committee, onQCCreated hotstuff.OnQCCreated) *VoteProcessorFactory {
	return &VoteProcessorFactory{
		base: &CombinedBaseVoteProcessorFactory{
			log:         log,
			committee:   committee,
			packer:      signature.NewConsensusSigDataPacker(committee),
			onQCCreated: onQCCreated,
		},
	}
}

// Create creates CombinedVoteProcessor using base factory for processing votes for current proposal.
// After constructing CombinedVoteProcessor validity of proposer vote is checked.
// Caller can be sure that proposal vote was verified and processed.
// Expected error returns during normal operations:
// * model.InvalidBlockError - proposal has invalid proposer vote
func (f *VoteProcessorFactory) Create(proposal *model.Proposal) (hotstuff.VerifyingVoteProcessor, error) {
	processor, err := f.base.Create(proposal)
	if err != nil {
		return nil, fmt.Errorf("could not create vote processor for block %v: %w", proposal.Block.BlockID, err)
	}

	err = processor.Process(proposal.ProposerVote())
	if err != nil {
		if model.IsInvalidVoteError(err) {
			return nil, model.InvalidBlockError{
				BlockID: proposal.Block.BlockID,
				View:    proposal.Block.View,
				Err:     err,
			}
		}
		return nil, fmt.Errorf("could not process proposer's vote from block %v: %w", proposal.Block.BlockID, err)
	}
	return processor, nil
}

// Create creates CombinedVoteProcessor for processing votes for current proposal.
// Caller must treat all errors as exceptions
func (f *CombinedBaseVoteProcessorFactory) Create(proposal *model.Proposal) (hotstuff.VerifyingVoteProcessor, error) {
	allParticipants, err := f.committee.Identities(proposal.Block.BlockID, filter.Any)
	if err != nil {
		return nil, fmt.Errorf("error retrieving consensus participants at block %v: %w", proposal.Block.BlockID, err)
	}

	// message that has to be verified against aggregated signature
	msg := verification.MakeVoteMessage(proposal.Block.View, proposal.Block.BlockID)

	stakingSigAggtor, err := signature.NewWeightedSignatureAggregator(allParticipants, msg, encoding.ConsensusVoteTag)
	if err != nil {
		return nil, fmt.Errorf("could not create aggregator for staking signatures: %w", err)
	}

	rbSigAggtor, err := signature.NewWeightedSignatureAggregator(allParticipants, msg, encoding.ConsensusVoteTag)
	if err != nil {
		return nil, fmt.Errorf("could not create aggregator for thershold signatures: %w", err)
	}

	rbRector := &signature.RandomBeaconReconstructor{} // TODO: initialize properly when ready

	minRequiredStake := hotstuff.ComputeStakeThresholdForBuildingQC(allParticipants.TotalStake())

	processor := newCombinedVoteProcessor(
		f.log,
		proposal.Block,
		stakingSigAggtor,
		rbSigAggtor,
		rbRector,
		f.onQCCreated,
		f.packer,
		minRequiredStake,
	)
	return processor, nil
}
