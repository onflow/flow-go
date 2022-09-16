package handler

import (
	"fmt"

	"github.com/onflow/cadence/runtime/common"
	"github.com/onflow/cadence/runtime/interpreter"

	"github.com/onflow/flow-go/fvm/state"
)

// TODO(patrick): remove and switch to *programs.TransactionPrograms once
// emulator is updated.
type TransactionPrograms interface {
	Get(loc common.Location) (*interpreter.Program, *state.State, bool)
	Set(loc common.Location, prog *interpreter.Program, state *state.State)
}

// ProgramsHandler manages operations using Programs storage.
// It's separation of concern for hostEnv
// Cadence contract guarantees that Get/Set methods will be called in a LIFO manner,
// so we use stack based approach here. During successful execution stack should be cleared
// naturally, making cleanup method essentially no-op. But if something goes wrong, all nested
// views must be merged in order to make sure they are recorded
type ProgramsHandler struct {
	masterState *state.StateHolder
	Programs    TransactionPrograms

	// NOTE: non-address programs are not reusable across transactions, hence
	// they are kept out of the shared program cache.
	nonAddressPrograms map[common.LocationID]*interpreter.Program
}

// NewProgramsHandler construts a new ProgramHandler
func NewProgramsHandler(programs TransactionPrograms, stateHolder *state.StateHolder) *ProgramsHandler {
	return &ProgramsHandler{
		masterState:        stateHolder,
		Programs:           programs,
		nonAddressPrograms: map[common.LocationID]*interpreter.Program{},
	}
}

func (h *ProgramsHandler) Set(location common.Location, program *interpreter.Program) error {
	// ignore empty locations
	if location == nil {
		return nil
	}

	address, ok := location.(common.AddressLocation)

	// program cache track only for AddressLocation, so for anything other
	// simply put a value
	if !ok {
		h.nonAddressPrograms[location.ID()] = program
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

	address, ok := location.(common.AddressLocation)

	// program cache track only for AddressLocation
	if !ok {
		prog, ok := h.nonAddressPrograms[location.ID()]
		return prog, ok
	}

	program, state, has := h.Programs.Get(address)
	if has {
		err := h.masterState.AttachAndCommitParseRestricted(state)
		if err != nil {
			panic(fmt.Sprintf("merge error while getting program, panic: %s", err))
		}
		return program, true
	}

	_, err := h.masterState.BeginParseRestrictedNestedTransaction(address)
	if err != nil {
		panic(err)
	}
	return nil, false
}
