package timeoutcollector

import "github.com/onflow/flow-go/consensus/hotstuff/model"

type TimeoutCollector struct {
}

func (TimeoutCollector) AddTimeout(timeout *model.TimeoutObject) error {
	panic("implement me")
}

func (TimeoutCollector) View() uint64 {
	panic("implement me")
}
