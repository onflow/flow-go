package votecollector

import (
	"errors"
	"fmt"

	"github.com/rs/zerolog"
	"go.uber.org/atomic"

	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/consensus/hotstuff/signature"
	"github.com/onflow/flow-go/consensus/hotstuff/verification"
	"github.com/onflow/flow-go/model/encoding"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
	msig "github.com/onflow/flow-go/module/signature"
)

/* ***************** Base-Factory for StakingVoteProcessor ****************** */

// stakingVoteProcessorFactoryBase implements a factory for creating StakingVoteProcessor
// holds needed dependencies to initialize StakingVoteProcessor.
// stakingVoteProcessorFactoryBase is used in collector cluster.
// CAUTION:
// this base factory only creates the VerifyingVoteProcessor for the given block.
// It does _not_ check the proposer's vote for its own block, i.e. it does _not_
// implement `hotstuff.VoteProcessorFactory`. This base factory should be wrapped
// by `votecollector.VoteProcessorFactory` which adds the logic to verify
// the proposer's vote (decorator pattern).
type stakingVoteProcessorFactoryBase struct {
	log         zerolog.Logger
	committee   hotstuff.Committee
	onQCCreated hotstuff.OnQCCreated
}

// Create creates StakingVoteProcessor for processing votes for the given block.
// Caller must treat all errors as exceptions
func (f *stakingVoteProcessorFactoryBase) Create(block *model.Block) (hotstuff.VerifyingVoteProcessor, error) {
	allParticipants, err := f.committee.Identities(block.BlockID, filter.Any)
	if err != nil {
		return nil, fmt.Errorf("error retrieving consensus participants: %w", err)
	}

	// message that has to be verified against aggregated signature
	msg := verification.MakeVoteMessage(block.View, block.BlockID)

	stakingSigAggtor, err := signature.NewWeightedSignatureAggregator(allParticipants, msg, encoding.CollectorVoteTag)
	if err != nil {
		return nil, fmt.Errorf("could not create aggregator for staking signatures: %w", err)
	}

	minRequiredStake := hotstuff.ComputeStakeThresholdForBuildingQC(allParticipants.TotalStake())

	return &StakingVoteProcessor{
		log:              f.log,
		block:            block,
		stakingSigAggtor: stakingSigAggtor,
		onQCCreated:      f.onQCCreated,
		minRequiredStake: minRequiredStake,
		done:             *atomic.NewBool(false),
	}, nil
}

/* ****************** StakingVoteProcessor Implementation ******************* */

// StakingVoteProcessor implements the hotstuff.VerifyingVoteProcessor interface.
// It processes hotstuff votes from a collector cluster, where participants vote
// in favour of a block by proving their staking key signature.
// Concurrency safe.
type StakingVoteProcessor struct {
	log              zerolog.Logger
	block            *model.Block
	stakingSigAggtor hotstuff.WeightedSignatureAggregator
	onQCCreated      hotstuff.OnQCCreated
	minRequiredStake uint64
	done             atomic.Bool
}

// Block returns block that is part of proposal that we are processing votes for.
func (p *StakingVoteProcessor) Block() *model.Block {
	return p.block
}

// Status returns status of this vote processor, it's always verifying.
func (p *StakingVoteProcessor) Status() hotstuff.VoteCollectorStatus {
	return hotstuff.VoteCollectorStatusVerifying
}

// Process performs processing of single vote in concurrent safe way. This
// function is implemented to be called by multiple goroutines at the same time.
// Supports processing of both staking and threshold signatures. Design of this
// function is event driven, as soon as we collect enough weight to create a QC
// we will immediately do this and submit it via callback for further processing.
// Expected error returns during normal operations:
// * VoteForIncompatibleBlockError - submitted vote for incompatible block
// * VoteForIncompatibleViewError - submitted vote for incompatible view
// * model.InvalidVoteError - submitted vote with invalid signature
// All other errors should be treated as exceptions.
func (p *StakingVoteProcessor) Process(vote *model.Vote) error {
	err := EnsureVoteForBlock(vote, p.block)
	if err != nil {
		return fmt.Errorf("received incompatible vote: %w", err)
	}

	// Vote Processing state machine
	if p.done.Load() {
		return nil
	}
	err = p.stakingSigAggtor.Verify(vote.SignerID, vote.SigData)
	if err != nil {
		if errors.Is(err, msig.ErrInvalidFormat) {
			return model.NewInvalidVoteErrorf(vote, "vote %x for view %d has invalid signature: %w", vote.ID(), vote.View, err)
		}
		return fmt.Errorf("internal error checking signature validity: %w", err)
	}

	if p.done.Load() {
		return nil
	}
	totalWeight, err := p.stakingSigAggtor.TrustedAdd(vote.SignerID, vote.SigData)
	if err != nil {
		return fmt.Errorf("adding the signature to staking aggregator failed: %w", err)
	}

	// checking of conditions for building QC are satisfied
	if totalWeight < p.minRequiredStake {
		return nil
	}

	// At this point, we have enough signatures to build a QC. Another routine
	// might just be at this point. To avoid duplicate work, only one routine can pass:
	if !p.done.CAS(false, true) {
		return nil
	}
	qc, err := p.buildQC()
	if err != nil {
		return fmt.Errorf("internal error constructing QC from votes: %w", err)
	}
	p.onQCCreated(qc)

	return nil
}

// buildQC performs aggregation of signatures when we have collected enough
// weight for building QC. This function is run only once by single worker.
// Any error should be treated as exception.
func (p *StakingVoteProcessor) buildQC() (*flow.QuorumCertificate, error) {
	stakingSigners, aggregatedStakingSig, err := p.stakingSigAggtor.Aggregate()
	if err != nil {
		return nil, fmt.Errorf("could not aggregate staking signature: %w", err)
	}

	return &flow.QuorumCertificate{
		View:      p.block.View,
		BlockID:   p.block.BlockID,
		SignerIDs: stakingSigners,
		SigData:   aggregatedStakingSig,
	}, nil
}
