package consensus

import (
	"fmt"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/consensus/recovery"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/storage"
)

// TODO: this needs to be integrated with proper configuration and bootstrapping.

// NewFollower instantiates the consensus follower and recovers its in-memory state of pending blocks.
// It receives the list `pending` containing _all_ blocks that
//   - have passed the compliance layer and stored in the protocol state
//   - descend from the latest finalized block
//   - are listed in ancestor-first order (i.e. for any block B ∈ pending, B's parent must
//     be listed before B, unless B's parent is the latest finalized block)
//
// CAUTION: all pending blocks are required to be valid (guaranteed if the block passed the compliance layer)
func NewFollower(log zerolog.Logger,
	mempoolMetrics module.MempoolMetrics,
	headers storage.Headers,
	updater module.Finalizer,
	notifier hotstuff.FollowerConsumer,
	rootHeader *flow.Header,
	rootQC *flow.QuorumCertificate,
	finalized *flow.Header,
	pending []*flow.Header,
) (*hotstuff.FollowerLoop, error) {
	forks, err := NewForks(finalized, headers, updater, notifier, rootHeader, rootQC)
	if err != nil {
		return nil, fmt.Errorf("could not initialize forks: %w", err)
	}

	// recover forks internal state (inserts all pending blocks)
	err = recovery.Recover(log, pending, recovery.ForksState(forks))
	if err != nil {
		return nil, fmt.Errorf("could not recover hotstuff follower state: %w", err)
	}

	// initialize the follower loop
	loop, err := hotstuff.NewFollowerLoop(log, mempoolMetrics, forks)
	if err != nil {
		return nil, fmt.Errorf("could not create follower loop: %w", err)
	}

	return loop, nil
}
