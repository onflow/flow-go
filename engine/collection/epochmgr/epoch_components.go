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
) *EpochComponents {
	components := &EpochComponents{
		state:             state,
		comp:              comp,
		sync:              sync,
		hotstuff:          hotstuff,
		voteAggregator:    voteAggregator,
		timeoutAggregator: timeoutAggregator,
	}

	builder := component.NewComponentManagerBuilder()
	// start new worker that will start child components and wait for them to finish
	builder.AddWorker(func(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
		// start vote and timeout aggregators, hotstuff will be started by compliance engine
		voteAggregator.Start(ctx)
		timeoutAggregator.Start(ctx)
		comp.Start(ctx)
		// wait until all components start
		<-util.AllReady(components.comp, components.sync, components.voteAggregator, components.timeoutAggregator)

		// signal that startup has finished, and we are ready to go
		ready()

		// wait for shutdown to be commenced
		<-ctx.Done()
		// wait for compliance engine and event loop to shut down
		<-util.AllDone(components.comp, components.sync, components.voteAggregator, components.timeoutAggregator)
	})
	components.ComponentManager = builder.Build()

	return components
}

type StartableEpochComponents struct {
	*EpochComponents
	cancel context.CancelFunc // used to stop the epoch components
}

func NewStartableEpochComponents(components *EpochComponents, cancel context.CancelFunc) *StartableEpochComponents {
	return &StartableEpochComponents{
		EpochComponents: components,
		cancel:          cancel,
	}
}
