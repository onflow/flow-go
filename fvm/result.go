package fvm

import (
	"errors"
	"fmt"

	"github.com/onflow/cadence"
	jsoncdc "github.com/onflow/cadence/encoding/json"
	"github.com/onflow/cadence/runtime"

	"github.com/dapperlabs/flow-go/model/flow"
)

type InvocationResult struct {
	ID      flow.Identifier
	Value   cadence.Value
	Events  []cadence.Event
	Logs    []string
	Error   FlowError
	GasUsed uint64
}

func (r InvocationResult) Succeeded() bool {
	return r.Error == nil
}

func (r InvocationResult) TransactionEvents(txIndex uint32) ([]flow.Event, error) {
	flowEvents := make([]flow.Event, len(r.Events))

	for i, event := range r.Events {
		payload, err := jsoncdc.Encode(event)
		if err != nil {
			return nil, fmt.Errorf("failed to encode event: %w", err)
		}

		flowEvents[i] = flow.Event{
			Type:             flow.EventType(event.EventType.ID()),
			TransactionID:    r.ID,
			TransactionIndex: txIndex,
			EventIndex:       uint32(i),
			Payload:          payload,
		}
	}

	return flowEvents, nil
}

func createInvocationResult(
	id flow.Identifier,
	value cadence.Value,
	events []cadence.Event,
	logs []string,
	err error,
) (*InvocationResult, error) {
	if err != nil {
		possibleRuntimeError := runtime.Error{}
		if errors.As(err, &possibleRuntimeError) {
			// runtime errors occur when the execution reverts
			return &InvocationResult{
				ID: id,
				Error: &CodeExecutionError{
					RuntimeError: possibleRuntimeError,
				},
				Logs: logs,
			}, nil
		}

		// other errors are unexpected and should be treated as fatal
		return nil, err
	}

	return &InvocationResult{
		ID:      id,
		Value:   value,
		Events:  events,
		Logs:    logs,
		GasUsed: 0,
	}, nil
}
