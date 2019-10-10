package types

type Event struct {
	ID string
	// Values is a map of all the parameters to the event, keys are parameter
	// names, values are the parameter values and must be primitive types.
	Values map[string]interface{}
}
