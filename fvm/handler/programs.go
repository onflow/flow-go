package handler

import (
	"fmt"

	"github.com/onflow/cadence/runtime/common"
	"github.com/onflow/cadence/runtime/interpreter"

	"github.com/onflow/flow-go/engine/execution/state/delta"
	"github.com/onflow/flow-go/fvm"
)

// ProgramsHandler manages operations using Programs storage.
// It's separation of concern for hostEnv

type stackEntry struct {
	view     *delta.View
	location common.Location
}

// Cadence contract guarantees that Get/Set methods will be called in a LIFO manner,
// so we use stack based approach here. During successful execution stack should be cleared
// naturally, making cleanup method essentially no-op. But if something goes wrong, all nested
// views must be merged in order to make sure they are recorded
type ProgramsHandler struct {
	masterView *delta.View
	viewsStack []stackEntry
	Programs   *fvm.Programs
}

func (h *ProgramsHandler) Set(location common.Location, program *interpreter.Program) error {
	if len(h.viewsStack) == 0 {
		return fmt.Errorf("views stack empty while set called, for location %s", location.String())
	}

	// pop
	last := h.viewsStack[len(h.viewsStack)-1]
	h.viewsStack = h.viewsStack[0 : len(h.viewsStack)-1]

	if last.location.ID() != location.ID() {
		return fmt.Errorf("set called for type %s while last get was for %s", location.String(), last.location.String())
	}

	h.Programs.Set(location, program, last.view)

	h.mergeView(last.view)

	return nil
}

func (h *ProgramsHandler) mergeView(view *delta.View) {
	if len(h.viewsStack) == 0 {
		// if this was last item, merge to the master view
		h.masterView.MergeView(view)
	} else {
		h.viewsStack[len(h.viewsStack)-1].view.MergeView(view)
	}
}

func (h *ProgramsHandler) Get(location common.Location) (*interpreter.Program, bool) {

	program, view, has := h.Programs.Get(location)
	if has {
		h.mergeView(view)
		return program, true
	}

	parentView := h.masterView
	if len(h.viewsStack) > 0 {
		parentView = h.viewsStack[len(h.viewsStack)-1].view
	}

	childView := parentView.NewChild()

	h.viewsStack = append(h.viewsStack, stackEntry{
		view:     childView,
		location: location,
	})

	return nil, false
}

func (h *ProgramsHandler) Cleanup() {

	stackLen := len(h.viewsStack)

	if stackLen == 0 {
		return
	}

	for i := stackLen; i > 0; i-- {
		entry := h.viewsStack[i]
		h.viewsStack[i-1].view.MergeView(entry.view)
	}
	h.masterView.MergeView(h.viewsStack[0].view)
}

func NewProgramsHandler(programs *fvm.Programs, view *delta.View) *ProgramsHandler {
	return &ProgramsHandler{
		masterView: nil,
		viewsStack: nil,
		Programs:   programs,
	}
}
