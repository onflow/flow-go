package epochmgr

import (
	"context"

	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/module/util"
	"github.com/onflow/flow-go/state/cluster"
)

// EpochComponents represents all dependencies for running an epoch.
type EpochComponents struct {
	*component.ComponentManager
	state             cluster.State
	comp              component.Component
	messageHub        component.Component
	sync              module.ReadyDoneAware
	hotstuff          module.HotStuff
	voteAggregator    hotstuff.VoteAggregator
	timeoutAggregator hotstuff.TimeoutAggregator
}

var _ component.Component = (*EpochComponents)(nil)

func NewEpochComponents(
	state cluster.State,
	comp component.Component,
	sync module.ReadyDoneAware,
	hotstuff module.HotStuff,
	voteAggregator hotstuff.VoteAggregator,
	timeoutAggregator hotstuff.TimeoutAggregator,
	messageHub component.Component,
) *EpochComponents {
	components := &EpochComponents{
		state:             state,
		comp:              comp,
		sync:              sync,
		hotstuff:          hotstuff,
		voteAggregator:    voteAggregator,
		timeoutAggregator: timeoutAggregator,
		messageHub:        messageHub,
	}

	builder := component.NewComponentManagerBuilder()
	// start new worker that will start child components and wait for them to finish
	builder.AddWorker(func(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
		// start components
		voteAggregator.Start(ctx)
		timeoutAggregator.Start(ctx)
		hotstuff.Start(ctx)
		comp.Start(ctx)
		messageHub.Start(ctx)
		// wait until all components start
		<-util.AllReady(
			components.hotstuff,
			components.comp,
			components.sync,
			components.voteAggregator,
			components.timeoutAggregator,
			components.messageHub,
		)

		// signal that startup has finished, and we are ready to go
		ready()

		// wait for shutdown to be commenced
		<-ctx.Done()
		// wait for compliance engine and event loop to shut down
		<-util.AllDone(
			components.hotstuff,
			components.comp,
			components.sync,
			components.voteAggregator,
			components.timeoutAggregator,
			components.messageHub,
		)
	})
	components.ComponentManager = builder.Build()

	return components
}

// RunningEpochComponents contains all consensus-related components for an epoch
// and the cancel function to stop these components. All components must have been
// started when the RunningEpochComponents is constructed.
type RunningEpochComponents struct {
	*EpochComponents
	cancel context.CancelFunc // used to stop the epoch components
}

// NewRunningEpochComponents returns a new RunningEpochComponents container for the
// given epoch components. The components must have already been started using some
// context, which the provided cancel function cancels (stopping the epoch components).
func NewRunningEpochComponents(components *EpochComponents, cancel context.CancelFunc) *RunningEpochComponents {
	return &RunningEpochComponents{
		EpochComponents: components,
		cancel:          cancel,
	}
}
