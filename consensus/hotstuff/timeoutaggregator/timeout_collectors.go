package timeoutaggregator

import (
	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/module/component"
)

type TimeoutCollectors struct {
	*component.ComponentManager
}

var _ hotstuff.TimeoutCollectors = (*TimeoutCollectors)(nil)

func (t TimeoutCollectors) GetOrCreateCollector(view uint64) (collector hotstuff.TimeoutCollector, created bool, err error) {
	//TODO implement me
	panic("implement me")
}

func (t TimeoutCollectors) PruneUpToView(lowestRetainedView uint64) {
	//TODO implement me
	panic("implement me")
}
