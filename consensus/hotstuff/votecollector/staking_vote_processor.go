package votecollector

import (
	"errors"
	"fmt"

	"github.com/rs/zerolog"
	"go.uber.org/atomic"

	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/model/flow"
	msig "github.com/onflow/flow-go/module/signature"
)

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

// newStakingVoteProcessor is a helper function to perform object construction
// no extra logic for validating proposal wasn't added
func newStakingVoteProcessor(
	log zerolog.Logger,
	block *model.Block,
	stakingSigAggtor hotstuff.WeightedSignatureAggregator,
	onQCCreated hotstuff.OnQCCreated,
	minRequiredStake uint64,
) *StakingVoteProcessor {
	return &StakingVoteProcessor{
		log:              log,
		block:            block,
		stakingSigAggtor: stakingSigAggtor,
		onQCCreated:      onQCCreated,
		minRequiredStake: minRequiredStake,
		done:             *atomic.NewBool(false),
	}
}

// Block returns block that is part of proposal that we are processing votes for.
func (p *StakingVoteProcessor) Block() *model.Block {
	return p.block
}

// Status returns status of this vote processor, it's always verifying.
func (p *StakingVoteProcessor) Status() hotstuff.VoteCollectorStatus {
	return hotstuff.VoteCollectorStatusVerifying
}

// Process performs processing of single vote in concurrent safe way. This function is implemented to be
// called by multiple goroutines at the same time. Supports processing of both staking and threshold signatures.
// Design of this function is event driven, as soon as we collect enough weight to create a QC we will immediately do this
// and submit it via callback for further processing.
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
			return model.NewInvalidVoteErrorf(vote, "submitted invalid signature for vote (%x) at view %d", vote.ID(), vote.View)
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

// buildQC performs aggregation of signatures when we have collected enough weight
// for building QC. This function is run only once by single worker.
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
