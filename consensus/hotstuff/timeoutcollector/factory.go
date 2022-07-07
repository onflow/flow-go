package timeoutcollector

import (
	"fmt"

	"github.com/onflow/flow-go/consensus/hotstuff"
)

type Factory struct {
	notifier          hotstuff.Consumer
	collectorNotifier hotstuff.TimeoutCollectorConsumer
	validator         hotstuff.Validator
	committee         hotstuff.Replicas
	encodingTag       string
}

func (f *Factory) Create(view uint64) (hotstuff.TimeoutCollector, error) {
	allParticipants, err := f.committee.IdentitiesByEpoch(view)
	if err != nil {
		return nil, fmt.Errorf("error retrieving consensus participants: %w", err)
	}

	sigAggregator, err := NewTimeoutSignatureAggregator(view, allParticipants, f.encodingTag)
	if err != nil {
		return nil, fmt.Errorf("could not create TimeoutSignatureAggregator at view %d: %w", view, err)
	}

	processor, err := NewTimeoutProcessor(f.committee, f.validator, sigAggregator, f.collectorNotifier)
	if err != nil {
		return nil, fmt.Errorf("could not create TimeoutProcessor at view %d: %w", view, err)
	}

	return NewTimeoutCollector(view, f.notifier, f.collectorNotifier, processor), nil
}
