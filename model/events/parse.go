package events

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

// ParseEvent parses an event type into its parts. There are 3 valid EventType formats:
// - flow.[EventName]
// - evm.[EventName]
// - A.[Address].[Contract].[EventName]
// Any other format results in an error.
func ParseEvent(eventType flow.EventType) (*ParsedEvent, error) {
	parts := strings.Split(string(eventType), ".")

	switch parts[0] {
	case "flow", flow.EVMLocationPrefix:
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

// ValidateEvent validates an event type is properly formed and for the correct network, and returns
// a parsed event. If the event type is invalid, an error is returned.
func ValidateEvent(eventType flow.EventType, chain flow.Chain) (*ParsedEvent, error) {
	parsed, err := ParseEvent(eventType)
	if err != nil {
		return nil, err
	}

	// only account type events have an address field
	if parsed.Type != AccountEventType {
		return parsed, nil
	}

	contractAddress := flow.HexToAddress(parsed.Address)
	if !chain.IsValid(contractAddress) {
		return nil, fmt.Errorf("invalid event contract address")
	}

	return parsed, nil
}
