package timeoutaggregator

import (
	"fmt"
	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/consensus/hotstuff/timeoutcollector"
)

type TimeoutCollectorFactory struct {
	notifier          hotstuff.Consumer
	collectorNotifier hotstuff.TimeoutCollectorConsumer
	processorFactory  hotstuff.TimeoutProcessorFactory
}

func NewTimeoutCollectorFactory(notifier hotstuff.Consumer,
	collectorNotifier hotstuff.TimeoutCollectorConsumer,
	createProcessor hotstuff.TimeoutProcessorFactory,
) *TimeoutCollectorFactory {
	return &TimeoutCollectorFactory{
		notifier:          notifier,
		collectorNotifier: collectorNotifier,
		processorFactory:  createProcessor,
	}
}

func (f *TimeoutCollectorFactory) Create(view uint64) (hotstuff.TimeoutCollector, error) {
	processor, err := f.processorFactory.Create(view)
	if err != nil {
		return nil, fmt.Errorf("could not create TimeoutProcessor at view %d: %w", view, err)
	}
	return timeoutcollector.NewTimeoutCollector(view, f.notifier, f.collectorNotifier, processor), nil
}
