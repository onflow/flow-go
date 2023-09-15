package request

import (
	"fmt"
	"regexp"
)

type EventType string

var basicEventRe = regexp.MustCompile(`[A-Z]\.[a-f0-9]{16}\.[\w+]*\.[\w+]*`)
var flowEventRe = regexp.MustCompile(`flow\.[\w]*`)

func (e *EventType) Parse(raw string) error {
	if !basicEventRe.MatchString(raw) && !flowEventRe.MatchString(raw) {
		return fmt.Errorf("invalid event type format")
	}
	*e = EventType(raw)
	return nil
}

func (e EventType) Flow() string {
	return string(e)
}

type EventTypes []EventType

func (e *EventTypes) Parse(raw []string) error {
	// make a map to have only unique values as keys
	eventTypes := make(EventTypes, 0)
	uniqueTypes := make(map[string]bool)
	for i, r := range raw {
		var eType EventType
		err := eType.Parse(r)
		if err != nil {
			return fmt.Errorf("error at index %d: %w", i, err)
		}

		if !uniqueTypes[eType.Flow()] {
			uniqueTypes[eType.Flow()] = true
			eventTypes = append(eventTypes, eType)
		}
	}

	*e = eventTypes
	return nil
}

func (e EventTypes) Flow() []string {
	eventTypes := make([]string, len(e))
	for j, eType := range e {
		eventTypes[j] = eType.Flow()
	}
	return eventTypes
}
