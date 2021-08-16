//nolint
package votecollector

import (
	"sync/atomic"

	"github.com/onflow/flow-go/consensus/hotstuff"
)

type ConsensusVerifyingVoteCollector struct {
	done                atomic.Value                       // indicate whether we've collected enough votes
	validator           hotstuff.SigValidator              // the signature validator
	votedStakes         atomic.Value                       // the total stakes represented by all the voters.
	stakingAgg          hotstuff.SignatureAggregator       // store staking sig shares and aggregates staking sig
	beaconAgg           hotstuff.SignatureAggregator       // store random beacon sig shares and aggregates them as staking sigs
	beaconReconstructor hotstuff.RandomBeaconReconstructor // store random beacon sig shares and reconstruct the random beacon sig.
}
