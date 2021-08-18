// nolint
package votecollector

import (
	"fmt"

	"go.uber.org/atomic"

	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/consensus/hotstuff/sigvalidator"
	"github.com/onflow/flow-go/model/flow"
)

type ConsensusClusterVoteCollector struct {
	CollectionBase

	validator     *sigvalidator.ConsensusSigValidator
	combinedAggr  hotstuff.CombinedSigAggregator
	reconstructor hotstuff.RandomBeaconReconstructor
	onQCCreated   hotstuff.OnQCCreated
	done          atomic.Bool
}

func NewConsensusClusterVoteCollector(base CollectionBase) *ConsensusClusterVoteCollector {
	return &ConsensusClusterVoteCollector{
		CollectionBase: base,
	}
}

// CreateVote implements BlockSigner interface for creating votes from block proposals
func (c *ConsensusClusterVoteCollector) CreateVote(block *model.Block) (*model.Vote, error) {
	panic("implement me")
}

func (c *ConsensusClusterVoteCollector) AddVote(vote *model.Vote) error {
	if c.done.Load() {
		return nil
	}

	sigType, err := c.validator.ValidateVote(vote)
	// handle InvalidVoteError
	if err != nil {
		return fmt.Errorf("could not verify vote signature: %w", err)
	}

	if c.done.Load() {
		return nil
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
	if !c.hasSufficientStake() {
		return nil
	}

	if !c.reconstructor.HasSufficientShares() {
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

func (c *ConsensusClusterVoteCollector) hasSufficientStake() bool {
	panic("not implemented")
}

func (c *ConsensusClusterVoteCollector) Status() hotstuff.VoteCollectorStatus {
	return hotstuff.VoteCollectorStatusVerifying
}
