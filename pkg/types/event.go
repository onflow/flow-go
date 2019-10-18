package types

type Event struct {
	ID string
	// Values is a map of all the parameters to the event, keys are parameter
	// names, values are the parameter values and must be primitive types.
	Values map[string]interface{}
}

type EventQuery struct {
	// The event ID to search for. If empty, no filtering by ID is done.
	ID         string
	// The block to begin looking for events
	StartBlock uint64
	// The block to end looking for events (inclusive)
	EndBlock   uint64
}
