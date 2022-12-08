package environment

import (
	"github.com/onflow/cadence/runtime/interpreter"
)

type InterpreterSharedStateCache struct {
	cachedState *interpreter.SharedState
}

func NewInterpreterSharedStateCache() *InterpreterSharedStateCache {
	return &InterpreterSharedStateCache{}
}

// SetInterpreterSharedState sets the shared state of all interpreters.
func (c *InterpreterSharedStateCache) SetInterpreterSharedState(state *interpreter.SharedState) {
	// WARNING: setting c.cachedState to state fails tests, and is not safe.
	// This should be addressed within cadence.
	// c.cachedState = state
	c.cachedState = nil
}

// GetInterpreterSharedState gets the shared state of all interpreters.
// May return nil if none is available or use is not applicable.
func (c *InterpreterSharedStateCache) GetInterpreterSharedState() *interpreter.SharedState {
	return c.cachedState
}

func (c *InterpreterSharedStateCache) Reset() {
	c.cachedState = nil
}
