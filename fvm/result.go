package fvm

import (
	"fmt"

	"github.com/onflow/cadence"
	jsoncdc "github.com/onflow/cadence/encoding/json"
	"github.com/onflow/cadence/runtime"
	"github.com/onflow/cadence/runtime/interpreter"

	"github.com/dapperlabs/flow-go/model/flow"
)

type InvocationResult struct {
	ID      flow.Identifier
	Value   cadence.Value
	Events  []cadence.Event
	Logs    []string
	Error   Error
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
	vmErr, fatalErr := handleError(err)
	if fatalErr != nil {
		return nil, fatalErr
	}

	if vmErr != nil {
		return &InvocationResult{
			ID:    id,
			Error: vmErr,
			Logs:  logs,
			// TODO: https://github.com/dapperlabs/flow-go/issues/4139
			// Should gas be reported for failed transactions?
			GasUsed: 0,
		}, nil
	}

	return &InvocationResult{
		ID:     id,
		Value:  value,
		Events: events,
		Logs:   logs,
		// TODO: https://github.com/dapperlabs/flow-go/issues/4139
		GasUsed: 0,
	}, nil
}

func handleError(err error) (vmErr Error, fatalErr error) {
	switch typedErr := err.(type) {
	case runtime.Error:
		// If the error originated from the runtime, handle separately
		return handleRuntimeError(typedErr)
	case Error:
		// If the error is an fvm.Error, return as is
		return typedErr, nil
	default:
		// All other errors are considered fatal
		return nil, err
	}
}

func handleRuntimeError(err runtime.Error) (vmErr Error, fatalErr error) {
	innerErr := err.Err

	// External errors are reported by the runtime but originate from the VM.
	//
	// External errors may be fatal or non-fatal, so additional handling
	// is required.
	if externalErr, ok := innerErr.(interpreter.ExternalError); ok {
		if recoveredErr, ok := externalErr.Recovered.(error); ok {
			// If the recovered value is an error, pass it to the original
			// error handler to distinguish between fatal and non-fatal errors.
			return handleError(recoveredErr)
		}

		// If the recovered value is not an error, bubble up the panic.
		panic(externalErr.Recovered)
	}

	// All other errors are non-fatal Cadence errors.
	return &ExecutionError{Err: err}, nil
}
