package votecollector

import (
	"errors"
	"fmt"

	"github.com/rs/zerolog"
	"go.uber.org/atomic"

	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/consensus/hotstuff/signature"
	"github.com/onflow/flow-go/model/flow"
	msig "github.com/onflow/flow-go/module/signature"
)

// CombinedVoteProcessorFactory generates CombinedVoteProcessor instances
func CombinedVoteProcessorFactory(log zerolog.Logger, proposal *model.Proposal) (*CombinedVoteProcessor, error) {
	processor := &CombinedVoteProcessor{
		log:   log,
		block: proposal.Block,
		done:  *atomic.NewBool(false),
	}
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

// CombinedVoteProcessor implements the hotstuff.VerifyingVoteProcessor interface.
// It processes votes from the main consensus committee, where participants vote in
// favour of a block by proving either their staking key signature or their random
// beacon signature. In the former case, the participant only contributes to HotStuff
// progress; while in the latter case, the voter also contributes to running the
// random beacon. Concurrency safe.
type CombinedVoteProcessor struct {
	log              zerolog.Logger
	block            *model.Block
	stakingSigAggtor hotstuff.WeightedSignatureAggregator
	rbSigAggtor      hotstuff.WeightedSignatureAggregator
	rbRector         hotstuff.RandomBeaconReconstructor
	onQCCreated      hotstuff.OnQCCreated
	minRequiredStake uint64
	done             atomic.Bool
}

func (p *CombinedVoteProcessor) Block() *model.Block {
	return p.block
}

func (p *CombinedVoteProcessor) Status() hotstuff.VoteCollectorStatus {
	return hotstuff.VoteCollectorStatusVerifying
}

func (p *CombinedVoteProcessor) Process(vote *model.Vote) error {
	err := EnsureVoteForBlock(vote, p.block)
	if err != nil {
		return fmt.Errorf("received incompatible vote: %w", err)
	}

	// Vote Processing state machine
	if p.done.Load() {
		return nil
	}
	sigType, sig, err := signature.DecodeSingleSig(vote.SigData)
	if err != nil {
		if errors.Is(err, msig.ErrInvalidFormat) {
			return model.InvalidVoteError{VoteID: vote.ID(), View: vote.View, Err: err}
		}
		return fmt.Errorf("unexpected error decoding vote: %w", err)
	}

	switch sigType {

	case hotstuff.SigTypeStaking:
		valid, err := p.stakingSigAggtor.Verify(vote.SignerID, sig)
		if err != nil {
			return fmt.Errorf("internal error checking signature validity: %w", err)
		}
		if !valid {
			return model.NewInvalidVoteErrorf(vote, "submitted invalid signature for vote (%x) at view %d", vote.ID(), vote.View)
		}
		if p.done.Load() {
			return nil
		}
		_, err = p.stakingSigAggtor.TrustedAdd(vote.SignerID, sig)
		if err != nil {
			return fmt.Errorf("adding the signature to staking aggregator failed: %w", err)
		}

	case hotstuff.SigTypeRandomBeacon:
		valid, err := p.rbSigAggtor.Verify(vote.SignerID, sig)
		if err != nil {
			return fmt.Errorf("internal error checking signature validity: %w", err)
		}
		if !valid {
			return model.NewInvalidVoteErrorf(vote, "submitted invalid signature for vote (%x) at view %d", vote.ID(), vote.View)
		}
		if p.done.Load() {
			return nil
		}
		_, err = p.rbSigAggtor.TrustedAdd(vote.SignerID, sig)
		if err != nil {
			return fmt.Errorf("adding the signature to staking aggregator failed: %w", err)
		}
		_, err = p.rbRector.TrustedAdd(vote.SignerID, sig)
		if err != nil {
			return fmt.Errorf("adding the signature to random beacon reconstructor failed: %w", err)
		}

	default:
		return fmt.Errorf("invalid signature type %d: %w", sigType, msig.ErrInvalidFormat)
	}

	// checking of conditions for building QC are satisfied
	if p.stakingSigAggtor.TotalWeight()+p.rbSigAggtor.TotalWeight() < p.minRequiredStake {
		return nil
	}
	if !p.rbRector.HasSufficientShares() {
		return nil
	}

	// At this point, we have enough signatures to build a QC. Another routine
	// might just be at this point. To avoid uplicate work, only one routine can pass:
	if !p.done.CAS(false, true) {
		return nil
	}
	err = p.buildQC()
	if err != nil {
		return fmt.Errorf("could not build QC: %w", err)
	}

	return nil
}

func (p *CombinedVoteProcessor) buildQC() error {
	_, _, err := p.stakingSigAggtor.Aggregate()
	if err != nil {
		return fmt.Errorf("could not aggregate staking signature: %w", err)
	}
	_, _, err = p.rbSigAggtor.Aggregate()
	if err != nil {
		return fmt.Errorf("could not aggregate random beacon signatures: %w", err)
	}
	_, err = p.rbRector.Reconstruct()
	if err != nil {
		return fmt.Errorf("could not reconstruct random beacon group signature: %w", err)
	}

	// TODO: use signatures to build qc
	var qc *flow.QuorumCertificate
	p.onQCCreated(qc)

	panic("not implemented")
}
