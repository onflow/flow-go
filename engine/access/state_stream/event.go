package state_stream

import (
	"fmt"
	"strings"

	"github.com/onflow/flow-go/model/flow"
)

type ParsedEventType int

const (
	ProtocolEventType ParsedEventType = iota + 1
	AccountEventType
)

type ParsedEvent struct {
	Type         ParsedEventType
	EventType    flow.EventType
	Address      string
	Contract     string
	ContractName string
	Name         string
}

// ParseEvent parses an event type into its parts. There are 2 valid EventType formats:
// - flow.[EventName]
// - A.[Address].[Contract].[EventName]
// Any other format results in an error.
func ParseEvent(eventType flow.EventType) (*ParsedEvent, error) {
	parts := strings.Split(string(eventType), ".")

	switch parts[0] {
	case "flow":
		if len(parts) == 2 {
			return &ParsedEvent{
				Type:         ProtocolEventType,
				EventType:    eventType,
				Contract:     parts[0],
				ContractName: parts[0],
				Name:         parts[1],
			}, nil
		}

	case "A":
		if len(parts) == 4 {
			return &ParsedEvent{
				Type:         AccountEventType,
				EventType:    eventType,
				Address:      parts[1],
				Contract:     fmt.Sprintf("A.%s.%s", parts[1], parts[2]),
				ContractName: parts[2],
				Name:         parts[3],
			}, nil
		}
	}

	return nil, fmt.Errorf("invalid event type: %s", eventType)
}
