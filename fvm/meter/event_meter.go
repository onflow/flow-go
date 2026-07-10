package meter

import (
	"math"

	"github.com/onflow/flow-go/fvm/errors"
)

type EventMeterParameters struct {
	eventEmitByteLimit uint64
}

func DefaultEventMeterParameters() EventMeterParameters {
	return EventMeterParameters{
		eventEmitByteLimit: math.MaxUint64,
	}
}

func (params MeterParameters) WithEventEmitByteLimit(
	byteLimit uint64,
) MeterParameters {
	newParams := params
	newParams.eventEmitByteLimit = byteLimit
	return newParams
}

type EventMeter struct {
	params EventMeterParameters

	totalEmittedEventBytes uint64
}

func NewEventMeter(params EventMeterParameters) EventMeter {
	return EventMeter{
		params: params,
	}
}

// MeterEmittedEvent captures the byte size of an emitted event.
//
// Expected error returns during normal operation:
//   - [errors.LimitExceededError] with [errors.LimitKindEvent] if the total
//     byte size of emitted events exceeds the event byte size limit
func (m *EventMeter) MeterEmittedEvent(byteSize uint64) error {
	m.totalEmittedEventBytes += byteSize

	if m.totalEmittedEventBytes > m.params.eventEmitByteLimit {
		return errors.NewLimitExceededError(
			errors.LimitKindEvent,
			m.params.eventEmitByteLimit)
	}
	return nil
}

func (m *EventMeter) TotalEmittedEventBytes() uint64 {
	return m.totalEmittedEventBytes
}

func (m *EventMeter) Merge(child EventMeter) {
	m.totalEmittedEventBytes += child.TotalEmittedEventBytes()
}
