package environment

import (
	"fmt"

	"github.com/hashicorp/go-multierror"

	"github.com/onflow/cadence"
	jsoncdc "github.com/onflow/cadence/encoding/json"
	"github.com/onflow/cadence/runtime/common"
	"github.com/onflow/cadence/runtime/interpreter"

	"github.com/onflow/flow-go/fvm/derived"
	"github.com/onflow/flow-go/fvm/errors"
	"github.com/onflow/flow-go/fvm/state"
	"github.com/onflow/flow-go/fvm/storage"
	"github.com/onflow/flow-go/fvm/tracing"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/trace"
)

// Programs manages operations around cadence program parsing.
//
// Note that cadence guarantees that Get/Set methods are called in a LIFO
// manner. Hence, we create new nested transactions on Get calls and commit
// these nested transactions on Set calls in order to capture the states
// needed for parsing the programs.
type Programs struct {
	tracer  tracing.TracerSpan
	meter   Meter
	metrics MetricsReporter

	txnState storage.Transaction
	accounts Accounts

	// NOTE: non-address programs are not reusable across transactions, hence
	// they are kept out of the derived data database.
	nonAddressPrograms map[common.Location]*interpreter.Program

	// dependencyStack tracks programs currently being loaded and their dependencies.
	dependencyStack *dependencyStack
}

// NewPrograms constructs a new ProgramHandler
func NewPrograms(
	tracer tracing.TracerSpan,
	meter Meter,
	metrics MetricsReporter,
	txnState storage.Transaction,
	accounts Accounts,
) *Programs {
	return &Programs{
		tracer:             tracer,
		meter:              meter,
		metrics:            metrics,
		txnState:           txnState,
		accounts:           accounts,
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

	state, err := programs.txnState.CommitParseRestrictedNestedTransaction(
		address)
	if err != nil {
		return err
	}

	if state.BytesWritten() > 0 {
		// This should never happen. Loading a program should not write to the state.
		// If this happens, it indicates an implementation error.
		return fmt.Errorf("cannot set program. State was written to during program parsing")
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

	programs.txnState.SetProgram(address, &derived.Program{
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

	program, state, has := programs.txnState.GetProgram(address)
	if has {
		programs.cacheHit()

		programs.dependencyStack.addDependencies(program.Dependencies)
		err := programs.txnState.AttachAndCommitNestedTransaction(state)
		if err != nil {
			panic(fmt.Sprintf(
				"merge error while getting program, panic: %s",
				err))
		}

		return program.Program, true
	}
	programs.cacheMiss()

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

// GetOrLoadProgram gets the program from the cache,
// or loads it (by calling load) if it is not in the cache.
// When loading a program, this method will be re-entered
// to load the dependencies of the program.
func (programs *Programs) GetOrLoadProgram(
	location common.Location,
	load func() (*interpreter.Program, error),
) (*interpreter.Program, error) {
	// TODO: check why this exists and try to remove.
	// ignore empty locations
	if location == nil {
		return nil, nil
	}

	defer programs.tracer.StartChildSpan(trace.FVMEnvGetOrLoadProgram).End()
	err := programs.meter.MeterComputation(ComputationKindGetOrLoadProgram, 1)
	if err != nil {
		return nil, fmt.Errorf("get program failed: %w", err)
	}

	// non-address location program is not reusable across transactions.
	switch location := location.(type) {
	case common.AddressLocation:
		return programs.getOrLoadAddressProgram(location, load)
	default:
		return programs.getOrLoadNonAddressProgram(location, load)
	}
}

func (programs *Programs) getOrLoadAddressProgram(
	location common.AddressLocation,
	load func() (*interpreter.Program, error),
) (*interpreter.Program, error) {

	loader := newProgramLoader(load, programs.dependencyStack, location)
	program, err := programs.txnState.GetOrComputeProgram(
		programs.txnState,
		location,
		loader,
	)
	if err != nil {
		return nil, fmt.Errorf("error getting program: %w", err)
	}

	// Add dependencies to the stack.
	// This is only really needed if loader was not called,
	// but there is no harm in doing it always.
	programs.dependencyStack.addDependencies(program.Dependencies)

	if loader.Called() {
		programs.cacheMiss()
	} else {
		programs.cacheHit()
	}

	return program.Program, nil
}

// programLoader is used to load a program from a location.
type programLoader struct {
	loadFunc        func() (*interpreter.Program, error)
	dependencyStack *dependencyStack
	called          bool
	location        common.AddressLocation
}

var _ derived.ValueComputer[common.AddressLocation, *derived.Program] = (*programLoader)(nil)

func newProgramLoader(
	loadFunc func() (*interpreter.Program, error),
	dependencyStack *dependencyStack,
	location common.AddressLocation,
) *programLoader {
	return &programLoader{
		loadFunc:        loadFunc,
		dependencyStack: dependencyStack,
		// called will be true if the loader was called.
		called:   false,
		location: location,
	}
}

func (loader *programLoader) Compute(
	txState state.NestedTransaction,
	location common.AddressLocation,
) (
	*derived.Program,
	error,
) {
	if loader.called {
		// This should never happen, as the program loader is only called once per
		// program. The same loader is never reused. This is only here to make
		// this more apparent.
		panic("program loader called twice")
	}
	if loader.location != location {
		// This should never happen, as the program loader constructed specifically
		// to load one location once. This is only a sanity check.
		panic("program loader called with unexpected location")
	}

	loader.called = true

	interpreterProgram, dependencies, err :=
		loader.loadWithDependencyTracking(location, loader.loadFunc)
	if err != nil {
		return nil, fmt.Errorf("load program failed: %w", err)
	}

	return &derived.Program{
		Program:      interpreterProgram,
		Dependencies: dependencies,
	}, nil
}

func (loader *programLoader) Called() bool {
	return loader.called
}

func (loader *programLoader) loadWithDependencyTracking(
	address common.AddressLocation,
	load func() (*interpreter.Program, error),
) (
	*interpreter.Program,
	derived.ProgramDependencies,
	error,
) {
	// this program is not in cache, so we need to load it into the cache.
	// tho have proper invalidation, we need to track the dependencies of the program.
	// If this program depends on another program,
	// that program will be loaded before this one finishes loading (calls set).
	// That is why this is a stack.
	loader.dependencyStack.push(address)

	program, err := load()

	// Get collected dependencies of the loaded program.
	// Pop the dependencies from the stack even if loading errored.
	stackLocation, dependencies, depErr := loader.dependencyStack.pop()
	if depErr != nil {
		err = multierror.Append(err, depErr).ErrorOrNil()
	}

	if err != nil {
		return nil, nil, err
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
		return nil, nil, fmt.Errorf(
			"cannot set program. Popped dependencies are for an unexpeced address"+
				" (expected %s, got %s)", address, stackLocation)
	}
	return program, dependencies, nil
}

func (programs *Programs) getOrLoadNonAddressProgram(
	location common.Location,
	load func() (*interpreter.Program, error),
) (*interpreter.Program, error) {
	program, ok := programs.nonAddressPrograms[location]
	if ok {
		return program, nil
	}

	program, err := load()
	if err != nil {
		return nil, err
	}

	programs.nonAddressPrograms[location] = program
	return program, nil
}

func (programs *Programs) GetProgram(
	location common.Location,
) (
	*interpreter.Program,
	error,
) {
	defer programs.tracer.StartChildSpan(trace.FVMEnvGetProgram).End()

	err := programs.meter.MeterComputation(ComputationKindGetProgram, 1)
	if err != nil {
		return nil, fmt.Errorf("get program failed: %w", err)
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
	defer programs.tracer.StartChildSpan(trace.FVMEnvSetProgram).End()

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
	defer programs.tracer.StartExtensiveTracingChildSpan(
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

func (programs *Programs) cacheHit() {
	programs.metrics.RuntimeTransactionProgramsCacheHit()
}

func (programs *Programs) cacheMiss() {
	programs.metrics.RuntimeTransactionProgramsCacheMiss()
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
	dependencies.AddDependency(flow.ConvertAddress(loc.Address))

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
