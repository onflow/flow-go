package state

import (
	"fmt"

	"github.com/onflow/cadence/runtime/common"

	"github.com/onflow/flow-go/fvm/meter"
	"github.com/onflow/flow-go/model/flow"
)

// Opaque identifier used for Restarting nested transactions
type NestedTransactionId struct {
	state *ExecutionState
}

func (id NestedTransactionId) StateForTestingOnly() *ExecutionState {
	return id.state
}

type Meter interface {
	MeterComputation(kind common.ComputationKind, intensity uint) error
	ComputationIntensities() meter.MeteredComputationIntensities
	TotalComputationLimit() uint
	TotalComputationUsed() uint64

	MeterMemory(kind common.MemoryKind, intensity uint) error
	MemoryIntensities() meter.MeteredMemoryIntensities
	TotalMemoryEstimate() uint64

	InteractionUsed() uint64

	MeterEmittedEvent(byteSize uint64) error
	TotalEmittedEventBytes() uint64

	// RunWithAllLimitsDisabled runs f with limits disabled
	RunWithAllLimitsDisabled(f func())
}

// NestedTransaction provides active transaction states and facilitates common
// state management operations.
type NestedTransaction interface {
	Meter

	// NumNestedTransactions returns the number of uncommitted nested
	// transactions.  Note that the main transaction is not considered a
	// nested transaction.
	NumNestedTransactions() int

	// IsParseRestricted returns true if the current nested transaction is in
	// parse resticted access mode.
	IsParseRestricted() bool

	MainTransactionId() NestedTransactionId

	// IsCurrent returns true if the provide id refers to the current (nested)
	// transaction.
	IsCurrent(id NestedTransactionId) bool

	// BeginNestedTransaction creates a unrestricted nested transaction within
	// the current unrestricted (nested) transaction.  The meter parameters are
	// inherited from the current transaction.  This returns error if the
	// current nested transaction is program restricted.
	BeginNestedTransaction() (
		NestedTransactionId,
		error,
	)

	// BeginNestedTransactionWithMeterParams creates a unrestricted nested
	// transaction within the current unrestricted (nested) transaction, using
	// the provided meter parameters. This returns error if the current nested
	// transaction is program restricted.
	BeginNestedTransactionWithMeterParams(
		params meter.MeterParameters,
	) (
		NestedTransactionId,
		error,
	)

	// BeginParseRestrictedNestedTransaction creates a restricted nested
	// transaction within the current (nested) transaction.  The meter
	// parameters are inherited from the current transaction.
	BeginParseRestrictedNestedTransaction(
		location common.AddressLocation,
	) (
		NestedTransactionId,
		error,
	)

	// CommitNestedTransaction commits the changes in the current unrestricted
	// nested transaction to the parent (nested) transaction.  This returns
	// error if the expectedId does not match the current nested transaction.
	// This returns the committed execution snapshot otherwise.
	//
	// Note: The returned committed execution snapshot may be reused by another
	// transaction via AttachAndCommitNestedTransaction to update the
	// transaction bookkeeping, but the caller must manually invalidate the
	// state.
	// USE WITH EXTREME CAUTION.
	CommitNestedTransaction(
		expectedId NestedTransactionId,
	) (
		*ExecutionSnapshot,
		error,
	)

	// CommitParseRestrictedNestedTransaction commits the changes in the
	// current restricted nested transaction to the parent (nested)
	// transaction.  This returns error if the specified location does not
	// match the tracked location. This returns the committed execution
	// snapshot otherwise.
	//
	// Note: The returned committed execution snapshot may be reused by another
	// transaction via AttachAndCommitNestedTransaction to update the
	// transaction bookkeeping, but the caller must manually invalidate the
	// state.
	// USE WITH EXTREME CAUTION.
	CommitParseRestrictedNestedTransaction(
		location common.AddressLocation,
	) (
		*ExecutionSnapshot,
		error,
	)

	// PauseNestedTransaction detaches the current nested transaction from the
	// parent transaction, and returns the paused nested transaction state.
	// The paused nested transaction may be resume via Resume.
	//
	// WARNING: Pause and Resume are intended for implementing continuation
	// passing style behavior for the transaction executor, with the assumption
	// that the states accessed prior to pausing remain valid after resumption.
	// The paused nested transaction should not be reused across transactions.
	// IT IS NOT SAFE TO PAUSE A NESTED TRANSACTION IN GENERAL SINCE THAT
	// COULD LEAD TO PHANTOM READS.
	PauseNestedTransaction(
		expectedId NestedTransactionId,
	) (
		*ExecutionState,
		error,
	)

	// ResumeNestedTransaction attaches the paused nested transaction (state)
	// to the current transaction.
	ResumeNestedTransaction(pausedState *ExecutionState)

	// AttachAndCommitNestedTransaction commits the changes from the cached
	// nested transaction execution snapshot to the current (nested)
	// transaction.
	AttachAndCommitNestedTransaction(cachedSnapshot *ExecutionSnapshot) error

	// RestartNestedTransaction merges all changes that belongs to the nested
	// transaction about to be restart (for spock/meter bookkeeping), then
	// wipes its view changes.
	RestartNestedTransaction(
		id NestedTransactionId,
	) error

	Get(id flow.RegisterID) (flow.RegisterValue, error)

	Set(id flow.RegisterID, value flow.RegisterValue) error

	ViewForTestingOnly() View
}

type nestedTransactionStackFrame struct {
	state *ExecutionState

	// When nil, the subtransaction will have unrestricted access to the runtime
	// environment.  When non-nil, the subtransaction will only have access to
	// the parts of the runtime environment necessary for importing/parsing the
	// program, specifically, environment.ContractReader and
	// environment.Programs.
	parseRestriction *common.AddressLocation
}

type transactionState struct {
	// NOTE: The first frame is always the main transaction, and is not
	// poppable during the course of the transaction.
	nestedTransactions []nestedTransactionStackFrame
}

// NewTransactionState constructs a new state transaction which manages nested
// transactions.
func NewTransactionState(
	startView View,
	params StateParameters,
) NestedTransaction {
	startState := NewExecutionState(startView, params)
	return &transactionState{
		nestedTransactions: []nestedTransactionStackFrame{
			nestedTransactionStackFrame{
				state:            startState,
				parseRestriction: nil,
			},
		},
	}
}

func (s *transactionState) current() nestedTransactionStackFrame {
	return s.nestedTransactions[s.NumNestedTransactions()]
}

func (s *transactionState) currentState() *ExecutionState {
	return s.current().state
}

func (s *transactionState) NumNestedTransactions() int {
	return len(s.nestedTransactions) - 1
}

func (s *transactionState) IsParseRestricted() bool {
	return s.current().parseRestriction != nil
}

func (s *transactionState) MainTransactionId() NestedTransactionId {
	return NestedTransactionId{
		state: s.nestedTransactions[0].state,
	}
}

func (s *transactionState) IsCurrent(id NestedTransactionId) bool {
	return s.currentState() == id.state
}

func (s *transactionState) BeginNestedTransaction() (
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

func (s *transactionState) BeginNestedTransactionWithMeterParams(
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

func (s *transactionState) BeginParseRestrictedNestedTransaction(
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

func (s *transactionState) push(
	child *ExecutionState,
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

func (s *transactionState) pop(op string) (*ExecutionState, error) {
	if len(s.nestedTransactions) < 2 {
		return nil, fmt.Errorf("cannot %s the main transaction", op)
	}

	child := s.current()
	s.nestedTransactions = s.nestedTransactions[:len(s.nestedTransactions)-1]

	return child.state, nil
}

func (s *transactionState) mergeIntoParent() (*ExecutionSnapshot, error) {
	childState, err := s.pop("commit")
	if err != nil {
		return nil, err
	}

	childSnapshot := childState.Finalize()

	err = s.current().state.Merge(childSnapshot)
	if err != nil {
		return nil, err
	}

	return childSnapshot, nil
}

func (s *transactionState) CommitNestedTransaction(
	expectedId NestedTransactionId,
) (
	*ExecutionSnapshot,
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

func (s *transactionState) CommitParseRestrictedNestedTransaction(
	location common.AddressLocation,
) (
	*ExecutionSnapshot,
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

func (s *transactionState) PauseNestedTransaction(
	expectedId NestedTransactionId,
) (
	*ExecutionState,
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

func (s *transactionState) ResumeNestedTransaction(pausedState *ExecutionState) {
	s.push(pausedState, nil)
}

func (s *transactionState) AttachAndCommitNestedTransaction(
	cachedSnapshot *ExecutionSnapshot,
) error {
	return s.current().state.Merge(cachedSnapshot)
}

func (s *transactionState) RestartNestedTransaction(
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

	return s.currentState().DropChanges()
}

func (s *transactionState) Get(
	id flow.RegisterID,
) (
	flow.RegisterValue,
	error,
) {
	return s.currentState().Get(id)
}

func (s *transactionState) Set(
	id flow.RegisterID,
	value flow.RegisterValue,
) error {
	return s.currentState().Set(id, value)
}

func (s *transactionState) MeterComputation(
	kind common.ComputationKind,
	intensity uint,
) error {
	return s.currentState().MeterComputation(kind, intensity)
}

func (s *transactionState) MeterMemory(
	kind common.MemoryKind,
	intensity uint,
) error {
	return s.currentState().MeterMemory(kind, intensity)
}

func (s *transactionState) ComputationIntensities() meter.MeteredComputationIntensities {
	return s.currentState().ComputationIntensities()
}

func (s *transactionState) TotalComputationLimit() uint {
	return s.currentState().TotalComputationLimit()
}

func (s *transactionState) TotalComputationUsed() uint64 {
	return s.currentState().TotalComputationUsed()
}

func (s *transactionState) MemoryIntensities() meter.MeteredMemoryIntensities {
	return s.currentState().MemoryIntensities()
}

func (s *transactionState) TotalMemoryEstimate() uint64 {
	return s.currentState().TotalMemoryEstimate()
}

func (s *transactionState) InteractionUsed() uint64 {
	return s.currentState().InteractionUsed()
}

func (s *transactionState) MeterEmittedEvent(byteSize uint64) error {
	return s.currentState().MeterEmittedEvent(byteSize)
}

func (s *transactionState) TotalEmittedEventBytes() uint64 {
	return s.currentState().TotalEmittedEventBytes()
}

func (s *transactionState) ViewForTestingOnly() View {
	return s.currentState().View()
}

func (s *transactionState) RunWithAllLimitsDisabled(f func()) {
	s.currentState().RunWithAllLimitsDisabled(f)
}
