package request

import (
	"fmt"
	"regexp"
)

type EventType string

func (e *EventType) Parse(raw string) error {
	basic, _ := regexp.MatchString(`[A-Z]\.[a-f0-9]{16}\.[\w+]*\.[\w+]*`, raw)
	// match core events flow.event
	core, _ := regexp.MatchString(`flow\.[\w]*`, raw)
	if !core && !basic {
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
	if len(raw) > MaxIDsLength {
		return fmt.Errorf("at most %d event types can be requested at a time", MaxIDsLength)
	}

	// make a map to have only unique values as keys
	eventTypes := make(EventTypes, 0)
	uniqueTypes := make(map[string]bool)
	for _, r := range raw {
		var eType EventType
		err := eType.Parse(r)
		if err != nil {
			return err
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
