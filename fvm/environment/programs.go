package environment

import (
	"fmt"

	"github.com/hashicorp/go-multierror"
	"golang.org/x/xerrors"

	"github.com/onflow/cadence"
	jsoncdc "github.com/onflow/cadence/encoding/json"
	"github.com/onflow/cadence/runtime/common"
	"github.com/onflow/cadence/runtime/interpreter"

	"github.com/onflow/flow-go/fvm/errors"
	"github.com/onflow/flow-go/fvm/storage"
	"github.com/onflow/flow-go/fvm/storage/derived"
	"github.com/onflow/flow-go/fvm/storage/state"
	"github.com/onflow/flow-go/fvm/tracing"
	"github.com/onflow/flow-go/module/trace"
)

type ProgramLoadingError struct {
	Err      error
	Location common.Location
}

func (p ProgramLoadingError) Unwrap() error {
	return p.Err
}

var _ error = ProgramLoadingError{}
var _ xerrors.Wrapper = ProgramLoadingError{}

func (p ProgramLoadingError) Error() string {
	return fmt.Sprintf("error getting program %v: %s", p.Location, p.Err)
}

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

	txnState storage.TransactionPreparer
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
	txnState storage.TransactionPreparer,
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

// Reset resets the program cache.
// this is called if the transactions happy path fails.
func (programs *Programs) Reset() {
	programs.nonAddressPrograms = make(map[common.Location]*interpreter.Program)
	programs.dependencyStack = newDependencyStack()
}

// GetOrLoadProgram gets the program from the cache,
// or loads it (by calling load) if it is not in the cache.
// When loading a program, this method will be re-entered
// to load the dependencies of the program.
func (programs *Programs) GetOrLoadProgram(
	location common.Location,
	load func() (*interpreter.Program, error),
) (*interpreter.Program, error) {
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
	top, err := programs.dependencyStack.top()
	if err != nil {
		return nil, err
	}

	if top.ContainsLocation(location) {
		// this dependency has already been seen in the current stack/scope
		// this means that it is safe to just fetch it and not reapply
		// state/metering changes
		program, ok := programs.txnState.GetProgram(location)
		if !ok {
			// program should be in the cache, if it is not,
			// this means there is an implementation error
			return nil, errors.NewDerivedDataCacheImplementationFailure(
				fmt.Errorf("expected program missing"+
					" in cache for location: %s", location))
		}
		err := programs.dependencyStack.add(program.Dependencies)
		if err != nil {
			return nil, err
		}
		programs.cacheHit()

		return program.Program, nil
	}

	loader := newProgramLoader(load, programs.dependencyStack, location)
	program, err := programs.txnState.GetOrComputeProgram(
		programs.txnState,
		location,
		loader,
	)
	if err != nil {
		return nil, ProgramLoadingError{
			Err:      err,
			Location: location,
		}
	}

	// Add dependencies to the stack.
	// This is only really needed if loader was not called,
	// but there is no harm in doing it always.
	err = programs.dependencyStack.add(program.Dependencies)
	if err != nil {
		return nil, err
	}

	if loader.Called() {
		programs.cacheMiss()
	} else {
		programs.cacheHit()
	}

	return program.Program, nil
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
	_ state.NestedTransactionPreparer,
	location common.AddressLocation,
) (
	*derived.Program,
	error,
) {
	if loader.called {
		// This should never happen, as the program loader is only called once per
		// program. The same loader is never reused. This is only here to make
		// this more apparent.
		return nil,
			errors.NewDerivedDataCacheImplementationFailure(
				fmt.Errorf("program loader called twice"))
	}

	if loader.location != location {
		// This should never happen, as the program loader constructed specifically
		// to load one location once. This is only a sanity check.
		return nil,
			errors.NewDerivedDataCacheImplementationFailure(
				fmt.Errorf("program loader called with unexpected location"))
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
	// to have proper invalidation, we need to track the dependencies of the program.
	// If this program depends on another program,
	// that program will be loaded before this one finishes loading (calls set).
	// That is why this is a stack.
	loader.dependencyStack.push(address)

	program, err := load()

	// Get collected dependencies of the loaded program.
	// Pop the dependencies from the stack even if loading errored.
	//
	// In case of an error, the dependencies of the errored program should not be merged
	// into the dependencies of the parent program. This is to prevent the parent program
	// from thinking that this program was already loaded and is in the cache,
	// if it requests it again.
	merge := err == nil
	stackLocation, dependencies, depErr := loader.dependencyStack.pop(merge)
	if depErr != nil {
		err = multierror.Append(err, depErr).ErrorOrNil()
	}

	if err != nil {
		return nil, derived.NewProgramDependencies(), err
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
		return nil, derived.NewProgramDependencies(), fmt.Errorf(
			"cannot set program. Popped dependencies are for an unexpeced address"+
				" (expected %s, got %s)", address, stackLocation)
	}
	return program, dependencies, nil
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
	location     common.Location
	dependencies derived.ProgramDependencies
}

// dependencyStack is a stack of dependencyTracker
// It is used during loading a program to create a dependency list for each program
type dependencyStack struct {
	trackers []dependencyTracker
}

func newDependencyStack() *dependencyStack {
	stack := &dependencyStack{
		trackers: make([]dependencyTracker, 0),
	}

	// The root of the stack is the program (script/transaction) that is being executed.
	// At the end of the transaction execution, this will hold all the dependencies
	// of the script/transaction.
	//
	// The root of the stack should never be popped.
	stack.push(common.StringLocation("^ProgramDependencyStackRoot$"))

	return stack
}

// push a new location to track dependencies for.
// it is assumed that the dependencies will be loaded before the program is set and pop is called.
func (s *dependencyStack) push(loc common.Location) {
	dependencies := derived.NewProgramDependencies()

	// A program is listed as its own dependency.
	dependencies.Add(loc)

	s.trackers = append(s.trackers, dependencyTracker{
		location:     loc,
		dependencies: dependencies,
	})
}

// add adds dependencies to the current dependency tracker
func (s *dependencyStack) add(dependencies derived.ProgramDependencies) error {
	l := len(s.trackers)
	if l == 0 {
		// This cannot happen, as the root of the stack is always present.
		return errors.NewDerivedDataCacheImplementationFailure(
			fmt.Errorf("dependency stack unexpectedly empty while calling add"))
	}

	s.trackers[l-1].dependencies.Merge(dependencies)
	return nil
}

// pop the last dependencies on the stack and return them.
// if merge is false then the dependencies are not merged into the parent tracker.
// this is used to pop the dependencies of a program that errored during loading.
func (s *dependencyStack) pop(merge bool) (common.Location, derived.ProgramDependencies, error) {
	if len(s.trackers) <= 1 {
		return nil,
			derived.NewProgramDependencies(),
			errors.NewDerivedDataCacheImplementationFailure(
				fmt.Errorf("cannot pop the programs" +
					" dependency stack, because it is empty"))
	}

	// pop the last tracker
	tracker := s.trackers[len(s.trackers)-1]
	s.trackers = s.trackers[:len(s.trackers)-1]

	if merge {
		// Add the dependencies of the popped tracker to the parent tracker
		// This is an optimisation to avoid having to iterate through the entire stack
		// everytime a dependency is pushed or added, instead we add the popped dependencies to the new top of the stack.
		// (because if C depends on B which depends on A, A's dependencies include C).
		s.trackers[len(s.trackers)-1].dependencies.Merge(tracker.dependencies)
	}

	return tracker.location, tracker.dependencies, nil
}

// top returns the last dependencies on the stack without pop-ing them.
func (s *dependencyStack) top() (derived.ProgramDependencies, error) {
	l := len(s.trackers)
	if l == 0 {
		// This cannot happen, as the root of the stack is always present.
		return derived.ProgramDependencies{}, errors.NewDerivedDataCacheImplementationFailure(
			fmt.Errorf("dependency stack unexpectedly empty while calling top"))
	}

	return s.trackers[len(s.trackers)-1].dependencies, nil
}
