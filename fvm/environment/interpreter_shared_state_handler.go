package environment

import (
	"github.com/onflow/cadence/runtime/interpreter"
)

type InterpreterSharedStateHandler struct {
	sharedState *interpreter.SharedState
}

func NewInterpreterSharedStateHandler() *InterpreterSharedStateHandler {
	return &InterpreterSharedStateHandler{}
}

func (h *InterpreterSharedStateHandler) SetInterpreterSharedState(state *interpreter.SharedState) {
	h.sharedState = state
}

func (h *InterpreterSharedStateHandler) GetInterpreterSharedState() *interpreter.SharedState {
	return h.sharedState
}

func (h *InterpreterSharedStateHandler) Reset() {
	h.sharedState = nil
}
