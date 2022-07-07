package timeoutcollector

import (
	"fmt"

	"github.com/onflow/flow-go/consensus/hotstuff"
)

type TimeoutProcessorFactory struct {
	committee   hotstuff.Replicas
	notifier    hotstuff.TimeoutCollectorConsumer
	validator   hotstuff.Validator
	encodingTag string
}

func NewTimeoutProcessorFactory(
	notifier hotstuff.TimeoutCollectorConsumer,
	committee hotstuff.Replicas,
	validator hotstuff.Validator,
	encodingTag string,
) *TimeoutProcessorFactory {
	return &TimeoutProcessorFactory{
		committee:   committee,
		notifier:    notifier,
		validator:   validator,
		encodingTag: encodingTag,
	}
}

func (f *TimeoutProcessorFactory) Create(view uint64) (hotstuff.TimeoutProcessor, error) {
	allParticipants, err := f.committee.IdentitiesByEpoch(view)
	if err != nil {
		return nil, fmt.Errorf("error retrieving consensus participants: %w", err)
	}

	sigAggregator, err := NewTimeoutSignatureAggregator(view, allParticipants, f.encodingTag)
	if err != nil {
		return nil, fmt.Errorf("could not create TimeoutSignatureAggregator at view %d: %w", view, err)
	}

	return NewTimeoutProcessor(f.committee, f.validator, sigAggregator, f.notifier)
}

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
	return NewTimeoutCollector(view, f.notifier, f.collectorNotifier, processor), nil
}
