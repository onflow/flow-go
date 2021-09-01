// nolint
package votecollector

import (
	"fmt"

	"go.uber.org/atomic"

	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/consensus/hotstuff/signature"
	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/model/flow"
)

type ConsensusClusterVoteCollector struct {
	CollectionBase

	block         *model.Block
	combinedAggr  hotstuff.CombinedSigAggregator
	reconstructor hotstuff.RandomBeaconReconstructor
	onQCCreated   hotstuff.OnQCCreated
	done          atomic.Bool
}

var _ hotstuff.VerifyingVoteCollector = &ConsensusClusterVoteCollector{}

// NewConsensusClusterVoteCollector creates new vote collector, accepts already validated proposal as argument which will
// be used as vote.
func NewConsensusClusterVoteCollector(base CollectionBase, proposal *model.Proposal) (*ConsensusClusterVoteCollector, error) {

	vote := proposal.ProposerVote()
	sigType, _, err := signature.DecodeSingleSig(vote.SigData)
	if err != nil {
		return nil, fmt.Errorf("could not parse vote signature type: %w", err)
	}

	clr := &ConsensusClusterVoteCollector{
		CollectionBase: base,
		block:          proposal.Block,
	}

	// process proposal as vote
	_, err = clr.combinedAggr.TrustedAdd(vote.SignerID, vote.SigData, sigType)
	if err != nil {
		return nil, fmt.Errorf("could not aggregate staking sig share: %w", err)
	}

	if sigType == hotstuff.SigTypeRandomBeacon {
		_, err = clr.reconstructor.TrustedAdd(vote.SignerID, vote.SigData)
		if err != nil {
			return nil, fmt.Errorf("could not add random beacon sig share: %w", err)
		}
	}

	return clr, nil
}

func (c *ConsensusClusterVoteCollector) Block() *model.Block {
	return c.block
}

// CreateVote implements BlockSigner interface for creating votes from block proposals
func (c *ConsensusClusterVoteCollector) CreateVote(block *model.Block) (*model.Vote, error) {
	panic("implement me")
}

func (c *ConsensusClusterVoteCollector) validateSignature(signerID flow.Identifier, sigType hotstuff.SigType, sig crypto.Signature) (bool, error) {
	switch sigType {
	case hotstuff.SigTypeStaking:
		return c.combinedAggr.Verify(signerID, sig, hotstuff.SigTypeStaking)
	case hotstuff.SigTypeRandomBeacon:
		return c.reconstructor.Verify(signerID, sig)
	}

	return false, fmt.Errorf("invalid sig type: %d", sigType)
}

func (c *ConsensusClusterVoteCollector) AddVote(vote *model.Vote) error {
	if c.done.Load() {
		return nil
	}

	// perform compliance checks on vote
	err := VoteComplianceCheck(vote, c.block)
	if err != nil {
		return fmt.Errorf("submitted invalid vote (%x) at view %d: %w", vote.ID(), vote.View, err)
	}

	sigType, sig, err := signature.DecodeSingleSig(vote.SigData)
	// handle InvalidVoteError
	if err != nil {
		return fmt.Errorf("could not parse vote signature type: %w", err)
	}

	valid, err := c.validateSignature(vote.SignerID, sigType, sig)
	if err != nil {
		return fmt.Errorf("could not verify vote signature: %w", err)
	}

	if !valid {
		return model.NewInvalidVoteErrorf(vote, "submitted invalid signature for vote (%x) at view %d", vote.ID(), vote.View)
	}

	if c.done.Load() {
		return nil
	}

	// after we have checked vote signature, let's track this vote to detect double voting
	err = c.doubleVoteDetector.TrustedAdd(vote)
	// either way if it's exception or a sentinel error we need to return with error
	if err != nil {
		return err
	}

	_, err = c.combinedAggr.TrustedAdd(vote.SignerID, vote.SigData, sigType)
	if err != nil {
		return fmt.Errorf("could not aggregate staking sig share: %w", err)
	}

	if sigType == hotstuff.SigTypeRandomBeacon {
		_, err = c.reconstructor.TrustedAdd(vote.SignerID, vote.SigData)
		if err != nil {
			return fmt.Errorf("could not add random beacon sig share: %w", err)
		}
	}

	// we haven't collected sufficient weight or shares, we have nothing to do further
	if !c.hasSufficientWeight() {
		return nil
	}

	if !c.reconstructor.HasSufficientShares() {
		return nil
	}
	if !c.done.CAS(false, true) {
		return nil
	}
	qc, err := c.buildQC()
	if err != nil {
		return fmt.Errorf("could not build QC: %w", err)
	}

	if qc != nil {
		c.onQCCreated(qc)
	}

	return nil
}

func (c *ConsensusClusterVoteCollector) buildQC() (*flow.QuorumCertificate, error) {
	// other goroutine might be constructing QC at this time, check with CAS
	// and exit early
	if !c.done.CAS(false, true) {
		return nil, nil
	}

	// at this point we can be sure that no one else is creating QC

	_, _, err := c.combinedAggr.Aggregate()
	if err != nil {
		return nil, fmt.Errorf("could not construct aggregated signatures: %w", err)
	}

	// reconstructor returns random beacon signature reconstructed from threshold signature shares
	_, err = c.reconstructor.Reconstruct()
	if err != nil {
		return nil, fmt.Errorf("could not reconstruct random beacon signature: %w", err)
	}

	// TODO: use signatures to build qc

	panic("not implemented")
}

func (c *ConsensusClusterVoteCollector) hasSufficientWeight() bool {
	panic("not implemented")
}

func (c *ConsensusClusterVoteCollector) Status() hotstuff.VoteCollectorStatus {
	return hotstuff.VoteCollectorStatusVerifying
}
