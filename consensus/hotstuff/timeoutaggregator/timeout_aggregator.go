package timeoutaggregator

import (
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/module/component"
)

type TimeoutAggregator struct {
	*component.ComponentManager
	log      zerolog.Logger
	notifier hotstuff.Consumer
}

var _ hotstuff.TimeoutAggregator = (*TimeoutAggregator)(nil)
var _ component.Component = (*TimeoutAggregator)(nil)

func (t TimeoutAggregator) AddTimeout(timeoutObject *model.TimeoutObject) {
	//TODO implement me
	panic("implement me")
}

func (t TimeoutAggregator) PruneUpToView(lowestRetainedView uint64) {
	//TODO implement me
	panic("implement me")
}
