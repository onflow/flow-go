package epoch

import (
	"fmt"

	"github.com/onflow/cadence"
	"github.com/onflow/cadence/encoding/json"

	"github.com/dapperlabs/flow-go/model/flow"
)

func ServiceEvent(event *flow.Event) (interface{}, error) {
	value, err := json.Decode(event.Payload)
	if err != nil {
		return nil, fmt.Errorf("could not event: %w", err)
	}
	switch event.Type {
	case flow.EventEpochSetup:
		return valueToSetup(value)
	case flow.EventEpochCommit:
		return valueToCommit(value)
	default:
		return nil, fmt.Errorf("invalid service event type (%s)", event.Type)
	}
}

func valueToSetup(value cadence.Value) (*Setup, error) {
	// TODO: figure out the cadence encoding (probably map) and
	// type assert all fields into the strongly typed struct
	return &Setup{}, nil
}

func valueToCommit(value cadence.Value) (*Commit, error) {
	// TODO: figure out the cadence encoding (probably map) and
	// type assert all fields into the strongly typed struct
	return &Commit{}, nil
}
