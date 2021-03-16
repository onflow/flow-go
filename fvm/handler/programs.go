package handler

import (
	"fmt"

	"github.com/onflow/cadence/runtime/common"
	"github.com/onflow/cadence/runtime/interpreter"

	"github.com/onflow/flow-go/fvm/programs"
	"github.com/onflow/flow-go/fvm/state"
)

func NewProgramsHandler(programs *programs.Programs, stateHolder *state.StateHolder) *ProgramsHandler {
	return &ProgramsHandler{
		masterState:  stateHolder,
		viewsStack:   nil,
		Programs:     programs,
		initialState: stateHolder.State(),
	}
}

// ProgramsHandler manages operations using Programs storage.
// It's separation of concern for hostEnv
type stackEntry struct {
	state    *state.State
	location common.Location
}

// Cadence contract guarantees that Get/Set methods will be called in a LIFO manner,
// so we use stack based approach here. During successful execution stack should be cleared
// naturally, making cleanup method essentially no-op. But if something goes wrong, all nested
// views must be merged in order to make sure they are recorded
type ProgramsHandler struct {
	masterState  *state.StateHolder
	viewsStack   []stackEntry
	Programs     *programs.Programs
	initialState *state.State
}

func (h *ProgramsHandler) Set(location common.Location, program *interpreter.Program) error {

	fmt.Printf("Set programs: %s\n", location.ID())

	// we track only for AddressLocation, so for anything other simply put a value
	if _, is := location.(common.AddressLocation); !is {
		h.Programs.Set(location, program, nil)
		return nil
	}

	if len(h.viewsStack) == 0 {
		return fmt.Errorf("views stack empty while set called, for location %s", location.String())
	}

	// pop
	last := h.viewsStack[len(h.viewsStack)-1]
	h.viewsStack = h.viewsStack[0 : len(h.viewsStack)-1]

	if last.location.ID() != location.ID() {
		return fmt.Errorf("set called for type %s while last get was for %s", location.String(), last.location.String())
	}

	h.Programs.Set(location, program, last.state)

	err := h.mergeState(last.state)

	return err
}

func (h *ProgramsHandler) mergeState(state *state.State) error {
	if len(h.viewsStack) == 0 {
		// if this was last item, merge to the master state

		fmt.Printf("Restoring active view NOOP from %p to: %p\n", h.masterState.State().View(), h.initialState.View())
		h.masterState.SetActiveState(h.initialState)
		//return h.masterState.State().MergeState(state)
	} else {
		fmt.Printf("Restoring active view from %p to: %p\n", h.masterState.State().View(), h.viewsStack[len(h.viewsStack)-1].state.View())

		h.masterState.SetActiveState(h.viewsStack[len(h.viewsStack)-1].state)
		//return h.viewsStack[len(h.viewsStack)-1].state.MergeState(state)
	}
	fmt.Printf("Merging %p to: %p\n", state.View(), h.masterState.State().View())

	return h.masterState.State().MergeState(state)

}

func (h *ProgramsHandler) Get(location common.Location) (*interpreter.Program, bool) {

	fmt.Printf("Get programs: %s\n", location.ID())

	program, view, has := h.Programs.Get(location)
	if has {
		if view != nil { // handle view not set (ie. for non-address locations
			err := h.mergeState(view)
			if err != nil {
				panic(fmt.Sprintf("merge error while getting program, panic: %s", err))
			}
		}
		return program, true
	}

	// we track only for AddressLocation
	if _, is := location.(common.AddressLocation); !is {
		return nil, false
	}

	parentState := h.masterState.State()
	if len(h.viewsStack) > 0 {
		parentState = h.viewsStack[len(h.viewsStack)-1].state
	}

	childState := parentState.NewChild()

	h.viewsStack = append(h.viewsStack, stackEntry{
		state:    childState,
		location: location,
	})

	fmt.Printf("Setting new active view from %p to: %p\n", h.masterState.State().View(), childState.View())
	h.masterState.SetActiveState(childState)

	return nil, false
}

func (h *ProgramsHandler) Cleanup() error {

	stackLen := len(h.viewsStack)

	if stackLen == 0 {
		return nil
	}

	for i := stackLen; i > 0; i-- {
		entry := h.viewsStack[i]
		err := h.viewsStack[i-1].state.MergeState(entry.state)
		if err != nil {
			return fmt.Errorf("cannot merge state while cleanup: %w", err)
		}
	}
	return h.masterState.State().MergeState(h.viewsStack[0].state)
}
