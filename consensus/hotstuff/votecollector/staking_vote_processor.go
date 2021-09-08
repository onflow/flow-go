//nolint
package votecollector

import (
	"fmt"

	"github.com/rs/zerolog"
	"go.uber.org/atomic"

	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/model/flow"
)

// StakingVoteProcessorFactory generates CombinedVoteProcessor instances
func StakingVoteProcessorFactory(log zerolog.Logger, proposal *model.Proposal) (*StakingVoteProcessor, error) {
	processor := &StakingVoteProcessor{
		log:   log,
		block: proposal.Block,
	}
	processor.done.Store(false)
	err := processor.Process(proposal.ProposerVote())
	if err != nil {
		if model.IsInvalidVoteError(err) {
			return nil, model.InvalidBlockError{
				BlockID: proposal.Block.BlockID,
				View:    proposal.Block.View,
				Err:     err,
			}
		}
		return nil, fmt.Errorf("")
	}
	return processor, nil
}

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

func (p *StakingVoteProcessor) Block() *model.Block {
	return p.block
}

func (p *StakingVoteProcessor) Status() hotstuff.VoteCollectorStatus {
	return hotstuff.VoteCollectorStatusVerifying
}

func (p *StakingVoteProcessor) Process(vote *model.Vote) error {
	err := EnsureVoteForBlock(vote, p.block)
	if err != nil {
		return fmt.Errorf("received incompatible vote: %w", err)
	}

	// Vote Processing state machine
	if p.done.Load() {
		return nil
	}
	valid, err := p.stakingSigAggtor.Verify(vote.SignerID, vote.SigData)
	if err != nil {
		return fmt.Errorf("internal error checking signature validity: %w", err)
	}
	if !valid {
		return model.NewInvalidVoteErrorf(vote, "submitted invalid signature for vote (%x) at view %d", vote.ID(), vote.View)
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
	err = p.buildQC()
	if err != nil {
		return fmt.Errorf("could not build QC: %w", err)
	}

	return nil
}

func (p *StakingVoteProcessor) buildQC() error {
	_, _, err := p.stakingSigAggtor.Aggregate()
	if err != nil {
		return fmt.Errorf("could not aggregate staking signature: %w", err)
	}

	// TODO: use signature to build qc
	var qc *flow.QuorumCertificate
	p.onQCCreated(qc)

	panic("not implemented")
}
