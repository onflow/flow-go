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

func NewFollower(log zerolog.Logger, committee hotstuff.Committee, headers storage.Headers, updater module.Finalizer,
	verifier hotstuff.Verifier, notifier hotstuff.FinalizationConsumer, rootHeader *flow.Header,
	rootQC *flow.QuorumCertificate, finalized *flow.Header, pending []*flow.Header) (*hotstuff.FollowerLoop, error) {

	finalizer, err := initFinalizer(finalized, headers, updater, notifier, rootHeader, rootQC)
	if err != nil {
		return nil, fmt.Errorf("could not initialize finalizer: %w", err)
	}

	// initialize the Validator
	validator := validator.New(committee, finalizer, verifier)

	// recover the hotstuff state as a follower
	err = recovery.Follower(log, finalizer, validator, finalized, pending)
	if err != nil {
		return nil, fmt.Errorf("could not recover hotstuff follower state: %w", err)
	}

	// initialize the follower logic
	logic, err := follower.New(log, validator, finalizer)
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
