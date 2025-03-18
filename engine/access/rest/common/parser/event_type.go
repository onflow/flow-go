package parser

import (
	"fmt"
	"regexp"
)

type EventType string

var basicEventRe = regexp.MustCompile(`[A-Z]\.[a-f0-9]{16}\.[\w+]*\.[\w+]*`)
var flowEventRe = regexp.MustCompile(`flow\.[\w]*`)

func NewEventType(raw string) (EventType, error) {
	if !basicEventRe.MatchString(raw) && !flowEventRe.MatchString(raw) {
		return "", fmt.Errorf("invalid event type format")
	}
	return EventType(raw), nil
}

func (e EventType) Flow() string {
	return string(e)
}

type EventTypes []EventType

func NewEventTypes(raw []string) (EventTypes, error) {
	eventTypes := make(EventTypes, 0)
	uniqueTypes := make(map[string]bool)
	for i, r := range raw {
		eType, err := NewEventType(r)
		if err != nil {
			return nil, fmt.Errorf("error at index %d: %w", i, err)
		}

		if !uniqueTypes[eType.Flow()] {
			uniqueTypes[eType.Flow()] = true
			eventTypes = append(eventTypes, eType)
		}
	}

	return eventTypes, nil
}

func (e EventTypes) Flow() []string {
	eventTypes := make([]string, len(e))
	for j, eType := range e {
		eventTypes[j] = eType.Flow()
	}
	return eventTypes
}
