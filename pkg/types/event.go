package types

import (
	"fmt"
	"strings"
)

type Event struct {
	ID string
	// Values is a map of all the parameters to the event, keys are parameter
	// names, values are the parameter values and must be primitive types.
	Values map[string]interface{}
}

// String returns the string representation of this event.
func (e Event) String() string {
	values := make([]string, len(e.Values))

	i := 0
	for key, value := range e.Values {
		values[i] = fmt.Sprintf("%s: %s", key, value)
		i++
	}

	return fmt.Sprintf("%s(%s)", e.ID, strings.Join(values, ", "))
}

type EventQuery struct {
	// The event ID to search for. If empty, no filtering by ID is done.
	ID string
	// The block to begin looking for events
	StartBlock uint64
	// The block to end looking for events (inclusive)
	EndBlock uint64
}
