package timeoutcollector

import (
	"fmt"

	"github.com/onflow/flow-go/consensus/hotstuff"
)

// TimeoutCollectorFactory implements hotstuff.TimeoutCollectorFactory, it is responsible for creating timeout collector
// for given view.
type TimeoutCollectorFactory struct {
	notifier          hotstuff.Consumer
	collectorNotifier hotstuff.TimeoutCollectorConsumer
	processorFactory  hotstuff.TimeoutProcessorFactory
}

var _ hotstuff.TimeoutCollectorFactory = (*TimeoutCollectorFactory)(nil)

// NewTimeoutCollectorFactory creates new instance of TimeoutCollectorFactory
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

// Create is a factory method to generate a TimeoutCollector for a given view
// Expected error returns during normal operations:
//  * model.ErrViewForUnknownEpoch if view is not yet pruned but no epoch containing the given view is known
// All other errors should be treated as exceptions.
func (f *TimeoutCollectorFactory) Create(view uint64) (hotstuff.TimeoutCollector, error) {
	processor, err := f.processorFactory.Create(view)
	if err != nil {
		return nil, fmt.Errorf("could not create TimeoutProcessor at view %d: %w", view, err)
	}
	return NewTimeoutCollector(view, f.notifier, f.collectorNotifier, processor), nil
}

type TimeoutProcessorFactory struct {
	committee   hotstuff.Replicas
	notifier    hotstuff.TimeoutCollectorConsumer
	validator   hotstuff.Validator
	encodingTag string
}

var _ hotstuff.TimeoutProcessorFactory = (*TimeoutProcessorFactory)(nil)

// NewTimeoutProcessorFactory creates new instance of TimeoutProcessorFactory
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

// Create is a factory method to generate a TimeoutProcessor for a given view
// Expected error returns during normal operations:
//  * model.ErrViewForUnknownEpoch no epoch containing the given view is known
// All other errors should be treated as exceptions.
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
