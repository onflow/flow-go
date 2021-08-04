package votecollector

import (
	"sync/atomic"

	"github.com/onflow/flow-go/consensus/hotstuff"
)

type ConsensusVerifyingVoteCollector struct {
	done                atomic.Value                    // indicate whether we've collected enough votes
	stakingAgg          hotstuff.StakingSigAggregator   // store staking sig shares and aggregates staking sig
	thresholdAgg        hotstuff.ThresholdSigAggregator // store threshold sig shares and aggregates threshold sigs as staking sigs
	votedStakes         atomic.Value
	beaconReconstructor hotstuff.RandomBeaconReconstructor // store random beacon sig shares and reconstruct the random beacon sig.
}
