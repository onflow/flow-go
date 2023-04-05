package consensus

import (
	"fmt"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/consensus/hotstuff/follower"
	"github.com/onflow/flow-go/consensus/hotstuff/validator"
	"github.com/onflow/flow-go/consensus/recovery"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/storage"
)

// TODO: this needs to be integrated with proper configuration and bootstrapping.

func NewFollower(log zerolog.Logger, committee hotstuff.DynamicCommittee, headers storage.Headers, updater module.Finalizer,
	verifier hotstuff.Verifier, notifier hotstuff.FinalizationConsumer, rootHeader *flow.Header,
	rootQC *flow.QuorumCertificate, finalized *flow.Header, pending []*flow.Header,
) (*hotstuff.FollowerLoop, error) {

	forks, err := NewForks(finalized, headers, updater, notifier, rootHeader, rootQC)
	if err != nil {
		return nil, fmt.Errorf("could not initialize forks: %w", err)
	}

	// initialize the Validator
	validator := validator.New(committee, verifier)

	// recover the HotStuff follower's internal state (inserts all pending blocks into Forks)
	err = recovery.Follower(log, forks, validator, pending)
	if err != nil {
		return nil, fmt.Errorf("could not recover hotstuff follower state: %w", err)
	}

	// initialize the follower logic
	logic, err := follower.New(log, validator, forks)
	if err != nil {
		return nil, fmt.Errorf("could not create follower logic: %w", err)
	}

	// initialize the follower loop
	loop, err := hotstuff.NewFollowerLoop(log, logic)
	if err != nil {
		return nil, fmt.Errorf("could not create follower loop: %w", err)
	}

	return loop, nil
}
