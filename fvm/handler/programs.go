package handler

import (
	"fmt"

	"github.com/onflow/cadence/runtime/common"
	"github.com/onflow/cadence/runtime/interpreter"

	"github.com/onflow/flow-go/fvm/programs"
	"github.com/onflow/flow-go/fvm/state"
)

// ProgramsHandler manages operations using Programs storage.
// It's separation of concern for hostEnv
// Cadence contract guarantees that Get/Set methods will be called in a LIFO manner,
// so we use stack based approach here. During successful execution stack should be cleared
// naturally, making cleanup method essentially no-op. But if something goes wrong, all nested
// views must be merged in order to make sure they are recorded
type ProgramsHandler struct {
	masterState *state.StateHolder
	Programs    *programs.TransactionPrograms
}

// NewProgramsHandler construts a new ProgramHandler
func NewProgramsHandler(programs *programs.TransactionPrograms, stateHolder *state.StateHolder) *ProgramsHandler {
	return &ProgramsHandler{
		masterState: stateHolder,
		Programs:    programs,
	}
}

func (h *ProgramsHandler) Set(location common.Location, program *interpreter.Program) error {
	// ignore empty locations
	if location == nil {
		return nil
	}

	address, ok := location.(common.AddressLocation)
	if !ok {
		h.Programs.Set(location, program, nil)
		return nil
	}

	state, err := h.masterState.CommitParseRestricted(address)
	if err != nil {
		return err
	}

	h.Programs.Set(address, program, state)
	return nil
}

func (h *ProgramsHandler) Get(location common.Location) (*interpreter.Program, bool) {
	// ignore empty locations
	if location == nil {
		return nil, false
	}

	program, state, has := h.Programs.Get(location)
	if has {
		if state != nil {
			err := h.masterState.AttachAndCommitParseRestricted(state)
			if err != nil {
				panic(fmt.Sprintf("merge error while getting program, panic: %s", err))
			}
		}

		return program, true
	}

	address, ok := location.(common.AddressLocation)
	if ok {
		_, err := h.masterState.BeginParseRestrictedNestedTransaction(address)
		if err != nil {
			panic(err)
		}
	}

	return nil, false
}
