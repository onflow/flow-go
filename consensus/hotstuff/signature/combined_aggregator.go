package signature

import (
	"fmt"
	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/model/flow"
	"go.uber.org/atomic"
)

var _ hotstuff.CombinedSigAggregator = &CombinedSigAggregatorImpl{}

type CombinedSigAggregatorImpl struct {
	stakingSigAggregator   hotstuff.SignatureAggregator
	thresholdSigAggregator hotstuff.SignatureAggregator
	stakeThreshold         uint64
	totalWeight            atomic.Uint64
}

func NewCombinedSigAggregator(totalStake uint64) *CombinedSigAggregatorImpl {
	return &CombinedSigAggregatorImpl{
		stakingSigAggregator:   nil,
		thresholdSigAggregator: nil,
		stakeThreshold:         hotstuff.ComputeStakeThresholdForBuildingQC(totalStake),
	}
}

func (c *CombinedSigAggregatorImpl) TrustedAdd(signer *flow.Identity, sig crypto.Signature, sigType hotstuff.SigType) (hasSufficientWeight bool, exception error) {
	var aggregator hotstuff.SignatureAggregator
	switch sigType {
	case hotstuff.SigTypeStaking:
		aggregator = c.stakingSigAggregator
	case hotstuff.SigTypeRandomBeacon:
		aggregator = c.thresholdSigAggregator
	default:
		return false, fmt.Errorf("invalid sig type")
	}

	_, err := aggregator.TrustedAdd(signer.ID(), sig)
	if err != nil {
		return false, fmt.Errorf("could not aggregate signature: %w", err)
	}

	accumulated := c.totalWeight.Add(signer.Stake)
	return accumulated > c.stakeThreshold, nil
}

func (c *CombinedSigAggregatorImpl) HasSufficientWeight() bool {
	return c.totalWeight.Load() > c.stakeThreshold
}

func (c *CombinedSigAggregatorImpl) Aggregate() ([]byte, []byte, error) {
	aggregatedStakingSig, err := c.stakingSigAggregator.Aggregate()
	if err != nil {
		return nil, nil, fmt.Errorf("could not aggregate staking signature: %w", err)
	}
	aggregatedThresholdSig, err := c.thresholdSigAggregator.Aggregate()
	if err != nil {
		return nil, nil, fmt.Errorf("could not aggregate threshold signature: %w", err)
	}

	return aggregatedStakingSig, aggregatedThresholdSig, nil
}
