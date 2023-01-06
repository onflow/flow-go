package environment

import (
	"fmt"

	"github.com/onflow/cadence"
	jsoncdc "github.com/onflow/cadence/encoding/json"
	"github.com/onflow/cadence/runtime/common"
	"github.com/onflow/cadence/runtime/interpreter"

	"github.com/onflow/flow-go/fvm/derived"
	"github.com/onflow/flow-go/fvm/errors"
	"github.com/onflow/flow-go/fvm/state"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/trace"
)

// TODO(patrick): remove and switch to *programs.DerivedTransactionData once
// https://github.com/onflow/flow-emulator/pull/229 is integrated.
type DerivedTransactionData interface {
	GetProgram(loc common.AddressLocation) (*derived.Program, *state.State, bool)
	SetProgram(loc common.AddressLocation, prog *derived.Program, state *state.State)
}

// Programs manages operations around cadence program parsing.
//
// Note that cadence guarantees that Get/Set methods are called in a LIFO
// manner. Hence, we create new nested transactions on Get calls and commit
// these nested transactions on Set calls in order to capture the states
// needed for parsing the programs.
type Programs struct {
	tracer *Tracer
	meter  Meter

	txnState *state.TransactionState
	accounts Accounts

	derivedTxnData DerivedTransactionData

	// NOTE: non-address programs are not reusable across transactions, hence
	// they are kept out of the derived data database.
	nonAddressPrograms map[common.Location]*interpreter.Program

	// dependencyStack tracks programs currently being loaded and their dependencies.
	dependencyStack *dependencyStack
}

// NewPrograms construts a new ProgramHandler
func NewPrograms(
	tracer *Tracer,
	meter Meter,
	txnState *state.TransactionState,
	accounts Accounts,
	derivedTxnData DerivedTransactionData,
) *Programs {
	return &Programs{
		tracer:             tracer,
		meter:              meter,
		txnState:           txnState,
		accounts:           accounts,
		derivedTxnData:     derivedTxnData,
		nonAddressPrograms: make(map[common.Location]*interpreter.Program),
		dependencyStack:    newDependencyStack(),
	}
}

func (programs *Programs) set(
	location common.Location,
	program *interpreter.Program,
) error {
	// ignore empty locations
	if location == nil {
		return nil
	}

	// derivedTransactionData only cache program/state for AddressLocation.
	// For non-address location, simply keep track of the program in the
	// environment.
	address, ok := location.(common.AddressLocation)
	if !ok {
		programs.nonAddressPrograms[location] = program
		return nil
	}

	state, err := programs.txnState.CommitParseRestricted(address)
	if err != nil {
		return err
	}

	// Get collected dependencies of the loaded program.
	stackLocation, dependencies, err := programs.dependencyStack.pop()
	if err != nil {
		return err
	}
	if stackLocation != address {
		// This should never happen, and indicates an implementation error.
		// GetProgram and SetProgram should be always called in pair, this check depends on this assumption.
		// Get pushes the stack and set pops the stack.
		// Example: if loading B that depends on A (and none of them are in cache yet),
		//   - get(A): pushes A
		//   - get(B): pushes B
		//   - set(B): pops B
		//   - set(A): pops A
		// Note: technically this check is redundant as `CommitParseRestricted` also has a similar check.
		return fmt.Errorf(
			"cannot set program. Popped dependencies are for an unexpeced address"+
				" (expected %s, got %s)", address, stackLocation)
	}

	programs.derivedTxnData.SetProgram(address, &derived.Program{
		Program:      program,
		Dependencies: dependencies,
	}, state)
	return nil
}

func (programs *Programs) get(
	location common.Location,
) (
	*interpreter.Program,
	bool,
) {
	// ignore empty locations
	if location == nil {
		return nil, false
	}

	address, ok := location.(common.AddressLocation)
	if !ok {
		program, ok := programs.nonAddressPrograms[location]
		return program, ok
	}

	program, state, has := programs.derivedTxnData.GetProgram(address)
	if has {
		programs.dependencyStack.addDependencies(program.Dependencies)
		err := programs.txnState.AttachAndCommit(state)
		if err != nil {
			panic(fmt.Sprintf(
				"merge error while getting program, panic: %s",
				err))
		}

		return program.Program, true
	}

	// this program is not in cache, so we need to load it into the cache.
	// tho have proper invalidation, we need to track the dependencies of the program.
	// If this program depends on another program,
	// that program will be loaded before this one finishes loading (calls set).
	// That is why this is a stack.
	programs.dependencyStack.push(address)

	// Address location program is reusable across transactions.  Create
	// a nested transaction here in order to capture the states read to
	// parse the program.
	_, err := programs.txnState.BeginParseRestrictedNestedTransaction(
		address)
	if err != nil {
		panic(err)
	}

	return nil, false
}

func (programs *Programs) GetProgram(
	location common.Location,
) (
	*interpreter.Program,
	error,
) {
	defer programs.tracer.StartSpanFromRoot(trace.FVMEnvGetProgram).End()

	err := programs.meter.MeterComputation(ComputationKindGetProgram, 1)
	if err != nil {
		return nil, fmt.Errorf("get program failed: %w", err)
	}

	if addressLocation, ok := location.(common.AddressLocation); ok {
		address := flow.Address(addressLocation.Address)

		freezeError := programs.accounts.CheckAccountNotFrozen(address)
		if freezeError != nil {
			return nil, fmt.Errorf("get program failed: %w", freezeError)
		}
	}

	program, has := programs.get(location)
	if has {
		return program, nil
	}

	return nil, nil
}

func (programs *Programs) SetProgram(
	location common.Location,
	program *interpreter.Program,
) error {
	defer programs.tracer.StartSpanFromRoot(trace.FVMEnvSetProgram).End()

	err := programs.meter.MeterComputation(ComputationKindSetProgram, 1)
	if err != nil {
		return fmt.Errorf("set program failed: %w", err)
	}

	err = programs.set(location, program)
	if err != nil {
		return fmt.Errorf("set program failed: %w", err)
	}
	return nil
}

func (programs *Programs) DecodeArgument(
	bytes []byte,
	_ cadence.Type,
) (
	cadence.Value,
	error,
) {
	defer programs.tracer.StartExtensiveTracingSpanFromRoot(
		trace.FVMEnvDecodeArgument).End()

	v, err := jsoncdc.Decode(programs.meter, bytes)
	if err != nil {
		return nil, fmt.Errorf(
			"decodeing argument failed: %w",
			errors.NewInvalidArgumentErrorf(
				"argument is not json decodable: %w",
				err))
	}

	return v, err
}

// dependencyTracker tracks dependencies for a location
// Or in other words it builds up a list of dependencies for the program being loaded.
// If a program imports another program (A imports B), then B is a dependency of A.
// Assuming that if A imports B which imports C (already in cache), the loading process looks like this:
//   - get(A): not in cache, so push A to tracker to start tracking dependencies for A.
//     We can be assured that set(A) will eventually be called.
//   - get(B): not in cache, push B
//   - get(C): in cache, do no push C, just add C's dependencies to the tracker (C's dependencies are also in the cache)
//   - set(B): pop B, getting all the collected dependencies for B, and add B's dependencies to the tracker
//     (because A also depends on everything B depends on)
//   - set(A): pop A, getting all the collected dependencies for A
type dependencyTracker struct {
	location     common.AddressLocation
	dependencies derived.ProgramDependencies
}

// dependencyStack is a stack of dependencyTracker
// It is used during loading a program to create a dependency list for each program
type dependencyStack struct {
	trackers []dependencyTracker
}

func newDependencyStack() *dependencyStack {
	return &dependencyStack{
		trackers: make([]dependencyTracker, 0),
	}
}

// push a new location to track dependencies for.
// it is assumed that the dependencies will be loaded before the program is set and pop is called.
func (s *dependencyStack) push(loc common.AddressLocation) {
	dependencies := make(derived.ProgramDependencies, 1)

	// A program is listed as its own dependency.
	dependencies.AddDependency(loc.Address)

	s.trackers = append(s.trackers, dependencyTracker{
		location:     loc,
		dependencies: dependencies,
	})
}

// addDependencies adds dependencies to the current dependency tracker
func (s *dependencyStack) addDependencies(dependencies derived.ProgramDependencies) {
	l := len(s.trackers)
	if l == 0 {
		// stack is empty.
		// This is expected if loading a program that is already cached.
		return
	}

	s.trackers[l-1].dependencies.Merge(dependencies)
}

// pop the last dependencies on the stack and return them.
func (s *dependencyStack) pop() (common.AddressLocation, derived.ProgramDependencies, error) {
	if len(s.trackers) == 0 {
		return common.AddressLocation{},
			nil,
			fmt.Errorf("cannot pop the programs dependency stack, because it is empty")
	}

	// pop the last tracker
	tracker := s.trackers[len(s.trackers)-1]
	s.trackers = s.trackers[:len(s.trackers)-1]

	// there are more trackers in the stack.
	// add the dependencies of the popped tracker to the parent tracker
	// This is an optimisation to avoid having to iterate through the entire stack
	// everytime a dependency is pushed or added, instead we add the popped dependencies to the new top of the stack.
	// (because if C depends on B which depends on A, A's dependencies include C).
	if len(s.trackers) > 0 {
		s.trackers[len(s.trackers)-1].dependencies.Merge(tracker.dependencies)
	}

	return tracker.location, tracker.dependencies, nil
}
