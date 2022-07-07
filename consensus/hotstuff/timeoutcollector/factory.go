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
