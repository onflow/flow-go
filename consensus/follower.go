package consensus

import (
	"fmt"

	"github.com/rs/zerolog"

	"github.com/dapperlabs/flow-go/consensus/hotstuff"
	"github.com/dapperlabs/flow-go/consensus/hotstuff/follower"
	"github.com/dapperlabs/flow-go/consensus/hotstuff/forks"
	"github.com/dapperlabs/flow-go/consensus/hotstuff/forks/finalizer"
	"github.com/dapperlabs/flow-go/consensus/hotstuff/model"
	"github.com/dapperlabs/flow-go/consensus/hotstuff/validator"
	"github.com/dapperlabs/flow-go/consensus/hotstuff/viewstate"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/module"
	"github.com/dapperlabs/flow-go/state/protocol"
)

// TODO: this needs to be integrated with proper configuration and bootstrapping.

func NewFollower(log zerolog.Logger, state protocol.State, me module.Local, updater module.Finalizer, verifier hotstuff.Verifier, notifier hotstuff.FinalizationConsumer, rootHeader *flow.Header, rootQC *model.QuorumCertificate, selector flow.IdentityFilter) (*hotstuff.FollowerLoop, error) {

	// initialize view state
	viewState, err := viewstate.New(state, me.NodeID(), selector)
	if err != nil {
		return nil, fmt.Errorf("could not initialize view state: %w", err)
	}

	// initialize internal finalizer
	rootBlock := &model.Block{
		View:        rootHeader.View,
		BlockID:     rootHeader.ID(),
		ProposerID:  rootHeader.ProposerID,
		QC:          nil,
		PayloadHash: rootHeader.PayloadHash,
		Timestamp:   rootHeader.Timestamp,
	}
	trustedRoot := &forks.BlockQC{
		QC:    rootQC,
		Block: rootBlock,
	}
	finalizer, err := finalizer.New(trustedRoot, updater, notifier)
	if err != nil {
		return nil, fmt.Errorf("could not initialize finalizer: %w", err)
	}

	// initialize the validator
	validator := validator.New(viewState, finalizer, verifier)

	// initialize the follower logic
	logic, err := follower.New(log, validator, finalizer, notifier)
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
