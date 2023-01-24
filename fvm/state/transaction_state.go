package state

import (
	"fmt"

	"github.com/onflow/cadence/runtime/common"

	"github.com/onflow/flow-go/fvm/meter"
	"github.com/onflow/flow-go/model/flow"
)

type nestedTransactionStackFrame struct {
	state *State

	// When nil, the subtransaction will have unrestricted access to the runtime
	// environment.  When non-nil, the subtransaction will only have access to
	// the parts of the runtime environment necessary for importing/parsing the
	// program, specifically, environment.ContractReader and
	// environment.Programs.
	parseRestriction *common.AddressLocation
}

// TransactionState provides active transaction states and facilitates common
// state management operations.
type TransactionState struct {
	// NOTE: The first frame is always the main transaction, and is not
	// poppable during the course of the transaction.
	nestedTransactions []nestedTransactionStackFrame
}

// Opaque identifier used for Restarting nested transactions
type NestedTransactionId struct {
	state *State
}

func (id NestedTransactionId) StateForTestingOnly() *State {
	return id.state
}

// NewTransactionState constructs a new state transaction which manages nested
// transactions.
func NewTransactionState(
	startView View,
	params StateParameters,
) *TransactionState {
	startState := NewState(startView, params)
	return &TransactionState{
		nestedTransactions: []nestedTransactionStackFrame{
			nestedTransactionStackFrame{
				state:            startState,
				parseRestriction: nil,
			},
		},
	}
}

func (s *TransactionState) current() nestedTransactionStackFrame {
	return s.nestedTransactions[s.NumNestedTransactions()]
}

func (s *TransactionState) currentState() *State {
	return s.current().state
}

// NumNestedTransactions returns the number of uncommitted nested transactions.
// Note that the main transaction is not considered a nested transaction.
func (s *TransactionState) NumNestedTransactions() int {
	return len(s.nestedTransactions) - 1
}

// IsParseRestricted returns true if the current nested transaction is in
// parse resticted access mode.
func (s *TransactionState) IsParseRestricted() bool {
	return s.current().parseRestriction != nil
}

func (s *TransactionState) MainTransactionId() NestedTransactionId {
	return NestedTransactionId{
		state: s.nestedTransactions[0].state,
	}
}

// IsCurrent returns true if the provide id refers to the current (nested)
// transaction.
func (s *TransactionState) IsCurrent(id NestedTransactionId) bool {
	return s.currentState() == id.state
}

// BeginNestedTransaction creates a unrestricted nested transaction within the
// current unrestricted (nested) transaction.  The meter parameters are
// inherited from the current transaction.  This returns error if the current
// nested transaction is program restricted.
func (s *TransactionState) BeginNestedTransaction() (
	NestedTransactionId,
	error,
) {
	if s.IsParseRestricted() {
		return NestedTransactionId{}, fmt.Errorf(
			"cannot begin a unrestricted nested transaction inside a " +
				"program restricted nested transaction",
		)
	}

	child := s.currentState().NewChild()
	s.push(child, nil)

	return NestedTransactionId{
		state: child,
	}, nil
}

// BeginNestedTransactionWithMeterParams creates a unrestricted nested
// transaction within the current unrestricted (nested) transaction, using the
// provided meter parameters. This returns error if the current nested
// transaction is program restricted.
func (s *TransactionState) BeginNestedTransactionWithMeterParams(
	params meter.MeterParameters,
) (
	NestedTransactionId,
	error,
) {
	if s.IsParseRestricted() {
		return NestedTransactionId{}, fmt.Errorf(
			"cannot begin a unrestricted nested transaction inside a " +
				"program restricted nested transaction",
		)
	}

	child := s.currentState().NewChildWithMeterParams(params)
	s.push(child, nil)

	return NestedTransactionId{
		state: child,
	}, nil
}

// BeginParseRestrictedNestedTransaction creates a restricted nested
// transaction within the current (nested) transaction.  The meter parameters
// are inherited from the current transaction.
func (s *TransactionState) BeginParseRestrictedNestedTransaction(
	location common.AddressLocation,
) (
	NestedTransactionId,
	error,
) {
	child := s.currentState().NewChild()
	s.push(child, &location)

	return NestedTransactionId{
		state: child,
	}, nil
}

func (s *TransactionState) push(
	child *State,
	location *common.AddressLocation,
) {
	s.nestedTransactions = append(
		s.nestedTransactions,
		nestedTransactionStackFrame{
			state:            child,
			parseRestriction: location,
		},
	)
}

func (s *TransactionState) pop(op string) (*State, error) {
	if len(s.nestedTransactions) < 2 {
		return nil, fmt.Errorf("cannot %s the main transaction", op)
	}

	child := s.current()
	s.nestedTransactions = s.nestedTransactions[:len(s.nestedTransactions)-1]

	return child.state, nil
}

func (s *TransactionState) mergeIntoParent() (*State, error) {
	childState, err := s.pop("commit")
	if err != nil {
		return nil, err
	}

	childState.committed = true

	err = s.current().state.MergeState(childState)
	if err != nil {
		return nil, err
	}

	return childState, nil
}

// Commit commits the changes in the current unrestricted nested transaction
// to the parent (nested) transaction.  This returns error if the expectedId
// does not match the current nested transaction.  This returns the committed
// state otherwise.
//
// Note: The returned committed state may be reused by another transaction via
// AttachAndCommit to update the transaction bookkeeping, but the caller must
// manually invalidate the state. USE WITH EXTREME CAUTION.
func (s *TransactionState) Commit(
	expectedId NestedTransactionId,
) (
	*State,
	error,
) {
	if !s.IsCurrent(expectedId) {
		return nil, fmt.Errorf(
			"cannot commit unexpected nested transaction: id mismatch",
		)
	}

	if s.IsParseRestricted() {
		// This is due to a programming error.
		return nil, fmt.Errorf(
			"cannot commit unexpected nested transaction: parse restricted",
		)
	}

	return s.mergeIntoParent()
}

// CommitParseRestricted commits the changes in the current restricted nested
// transaction to the parent (nested) transaction.  This returns error if the
// specified location does not match the tracked location.  This returns the
// committed state otherwise.
//
// Note: The returned committed state may be reused by another transaction via
// AttachAndCommit to update the transaction bookkeeping, but the caller must
// manually invalidate the state. USE WITH EXTREME CAUTION.
func (s *TransactionState) CommitParseRestricted(
	location common.AddressLocation,
) (
	*State,
	error,
) {
	currentFrame := s.current()
	if currentFrame.parseRestriction == nil ||
		*currentFrame.parseRestriction != location {

		// This is due to a programming error.
		return nil, fmt.Errorf(
			"cannot commit unexpected nested transaction %v != %v",
			currentFrame.parseRestriction,
			location,
		)
	}

	return s.mergeIntoParent()
}

// Pause detaches the current nested transaction from the parent transaction,
// and returns the paused nested transaction state.  The paused nested
// transaction may be resume via Resume.
//
// WARNING: Pause and Resume are intended for implementing continuation passing
// style behavior for the transaction executor, with the assumption that the
// states accessed prior to pausing remain valid after resumption.  The paused
// nested transaction should not be reused across transactions.  IT IS NOT
// SAFE TO PAUSE A NESTED TRANSACTION IN GENERAL SINCE THAT COULD LEAD TO
// PHANTOM READS.
func (s *TransactionState) Pause(
	expectedId NestedTransactionId,
) (
	*State,
	error,
) {
	if !s.IsCurrent(expectedId) {
		return nil, fmt.Errorf(
			"cannot pause unexpected nested transaction: id mismatch",
		)
	}

	if s.IsParseRestricted() {
		return nil, fmt.Errorf(
			"cannot Pause parse restricted nested transaction")
	}

	return s.pop("pause")
}

// Resume attaches the paused nested transaction (state) to the current
// transaction.
func (s *TransactionState) Resume(pausedState *State) {
	s.push(pausedState, nil)
}

// AttachAndCommit commits the changes in the cached nested transaction state
// to the current (nested) transaction.
func (s *TransactionState) AttachAndCommit(cachedState *State) error {
	s.push(cachedState, nil)
	_, err := s.mergeIntoParent()
	return err
}

// RestartNestedTransaction merges all changes that belongs to the nested
// transaction about to be restart (for spock/meter bookkeeping), then
// wipes its view changes.
func (s *TransactionState) RestartNestedTransaction(
	id NestedTransactionId,
) error {

	// NOTE: We need to verify the id is valid before any merge operation or
	// else we would accidently merge everything into the main transaction.
	found := false
	for _, frame := range s.nestedTransactions {
		if frame.state == id.state {
			found = true
			break
		}
	}

	if !found {
		return fmt.Errorf(
			"cannot restart nested transaction: nested transaction not found")
	}

	for s.currentState() != id.state {
		_, err := s.mergeIntoParent()
		if err != nil {
			return fmt.Errorf("cannot restart nested transaction: %w", err)
		}
	}

	s.currentState().View().DropDelta()
	return nil
}

func (s *TransactionState) Get(
	owner string,
	key string,
) (
	flow.RegisterValue,
	error,
) {
	return s.currentState().Get(owner, key)
}

func (s *TransactionState) Set(
	owner string,
	key string,
	value flow.RegisterValue,
) error {
	return s.currentState().Set(owner, key, value)
}

func (s *TransactionState) UpdatedAddresses() []flow.Address {
	return s.currentState().UpdatedAddresses()
}

func (s *TransactionState) MeterComputation(
	kind common.ComputationKind,
	intensity uint,
) error {
	return s.currentState().MeterComputation(kind, intensity)
}

func (s *TransactionState) MeterMemory(
	kind common.MemoryKind,
	intensity uint,
) error {
	return s.currentState().MeterMemory(kind, intensity)
}

func (s *TransactionState) ComputationIntensities() meter.MeteredComputationIntensities {
	return s.currentState().ComputationIntensities()
}

func (s *TransactionState) TotalComputationLimit() uint {
	return s.currentState().TotalComputationLimit()
}

func (s *TransactionState) TotalComputationUsed() uint64 {
	return s.currentState().TotalComputationUsed()
}

func (s *TransactionState) MemoryIntensities() meter.MeteredMemoryIntensities {
	return s.currentState().MemoryIntensities()
}

func (s *TransactionState) TotalMemoryEstimate() uint64 {
	return s.currentState().TotalMemoryEstimate()
}

func (s *TransactionState) InteractionUsed() uint64 {
	return s.currentState().InteractionUsed()
}

func (s *TransactionState) MeterEmittedEvent(byteSize uint64) error {
	return s.currentState().MeterEmittedEvent(byteSize)
}

func (s *TransactionState) TotalEmittedEventBytes() uint64 {
	return s.currentState().TotalEmittedEventBytes()
}

func (s *TransactionState) ViewForTestingOnly() View {
	return s.currentState().View()
}

func (s *TransactionState) UpdatedRegisterIDs() []flow.RegisterID {
	return s.currentState().UpdatedRegisterIDs()
}

func (s *TransactionState) UpdatedRegisters() flow.RegisterEntries {
	return s.currentState().UpdatedRegisters()
}

// RunWithAllLimitsDisabled runs f with limits disabled
func (s *TransactionState) RunWithAllLimitsDisabled(f func()) {
	s.currentState().RunWithAllLimitsDisabled(f)
}
