package state

import (
	"fmt"

	"github.com/onflow/cadence/runtime/common"

	"github.com/onflow/flow-go/fvm/meter"
	"github.com/onflow/flow-go/fvm/storage/snapshot"
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
	ComputationAvailable(kind common.ComputationKind, intensity uint) bool
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

// NestedTransactionPreparer provides active transaction states and facilitates
// common state management operations.
type NestedTransactionPreparer interface {
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

	// InterimReadSet returns the current read set aggregated from all
	// outstanding nested transactions.
	InterimReadSet() map[flow.RegisterID]struct{}

	// FinalizeMainTransaction finalizes the main transaction and returns
	// its execution snapshot.  The finalized main transaction will not accept
	// any new commits after this point.  This returns an error if there are
	// outstanding nested transactions.
	FinalizeMainTransaction() (*snapshot.ExecutionSnapshot, error)

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
		*snapshot.ExecutionSnapshot,
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
		*snapshot.ExecutionSnapshot,
		error,
	)

	// AttachAndCommitNestedTransaction commits the changes from the cached
	// nested transaction execution snapshot to the current (nested)
	// transaction.
	AttachAndCommitNestedTransaction(
		cachedSnapshot *snapshot.ExecutionSnapshot,
	) error

	// RestartNestedTransaction merges all changes that belongs to the nested
	// transaction about to be restart (for spock/meter bookkeeping), then
	// wipes its view changes.
	RestartNestedTransaction(
		id NestedTransactionId,
	) error

	Get(id flow.RegisterID) (flow.RegisterValue, error)

	Set(id flow.RegisterID, value flow.RegisterValue) error
}

type nestedTransactionStackFrame struct {
	*ExecutionState

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
	snapshot snapshot.StorageSnapshot,
	params StateParameters,
) NestedTransactionPreparer {
	startState := NewExecutionState(snapshot, params)
	return &transactionState{
		nestedTransactions: []nestedTransactionStackFrame{
			nestedTransactionStackFrame{
				ExecutionState:   startState,
				parseRestriction: nil,
			},
		},
	}
}

func (txnState *transactionState) current() nestedTransactionStackFrame {
	return txnState.nestedTransactions[txnState.NumNestedTransactions()]
}

func (txnState *transactionState) NumNestedTransactions() int {
	return len(txnState.nestedTransactions) - 1
}

func (txnState *transactionState) IsParseRestricted() bool {
	return txnState.current().parseRestriction != nil
}

func (txnState *transactionState) MainTransactionId() NestedTransactionId {
	return NestedTransactionId{
		state: txnState.nestedTransactions[0].ExecutionState,
	}
}

func (txnState *transactionState) IsCurrent(id NestedTransactionId) bool {
	return txnState.current().ExecutionState == id.state
}

func (txnState *transactionState) InterimReadSet() map[flow.RegisterID]struct{} {
	sizeEstimate := 0
	for _, frame := range txnState.nestedTransactions {
		sizeEstimate += frame.readSetSize()
	}

	result := make(map[flow.RegisterID]struct{}, sizeEstimate)

	// Note: the interim read set must be accumulated in reverse order since
	// the parent frame's write set will override the child frame's read set.
	for i := len(txnState.nestedTransactions) - 1; i >= 0; i-- {
		txnState.nestedTransactions[i].interimReadSet(result)
	}

	return result
}

func (txnState *transactionState) FinalizeMainTransaction() (
	*snapshot.ExecutionSnapshot,
	error,
) {
	if len(txnState.nestedTransactions) > 1 {
		return nil, fmt.Errorf(
			"cannot finalize with outstanding nested transaction(s)")
	}

	return txnState.nestedTransactions[0].Finalize(), nil
}

func (txnState *transactionState) BeginNestedTransaction() (
	NestedTransactionId,
	error,
) {
	if txnState.IsParseRestricted() {
		return NestedTransactionId{}, fmt.Errorf(
			"cannot begin a unrestricted nested transaction inside a " +
				"program restricted nested transaction",
		)
	}

	child := txnState.current().NewChild()
	txnState.push(child, nil)

	return NestedTransactionId{
		state: child,
	}, nil
}

func (txnState *transactionState) BeginNestedTransactionWithMeterParams(
	params meter.MeterParameters,
) (
	NestedTransactionId,
	error,
) {
	if txnState.IsParseRestricted() {
		return NestedTransactionId{}, fmt.Errorf(
			"cannot begin a unrestricted nested transaction inside a " +
				"program restricted nested transaction",
		)
	}

	child := txnState.current().NewChildWithMeterParams(params)
	txnState.push(child, nil)

	return NestedTransactionId{
		state: child,
	}, nil
}

func (txnState *transactionState) BeginParseRestrictedNestedTransaction(
	location common.AddressLocation,
) (
	NestedTransactionId,
	error,
) {
	child := txnState.current().NewChild()
	txnState.push(child, &location)

	return NestedTransactionId{
		state: child,
	}, nil
}

func (txnState *transactionState) push(
	child *ExecutionState,
	location *common.AddressLocation,
) {
	txnState.nestedTransactions = append(
		txnState.nestedTransactions,
		nestedTransactionStackFrame{
			ExecutionState:   child,
			parseRestriction: location,
		},
	)
}

func (txnState *transactionState) pop(op string) (*ExecutionState, error) {
	if len(txnState.nestedTransactions) < 2 {
		return nil, fmt.Errorf("cannot %s the main transaction", op)
	}

	child := txnState.current()
	txnState.nestedTransactions = txnState.nestedTransactions[:len(txnState.nestedTransactions)-1]

	return child.ExecutionState, nil
}

func (txnState *transactionState) mergeIntoParent() (
	*snapshot.ExecutionSnapshot,
	error,
) {
	childState, err := txnState.pop("commit")
	if err != nil {
		return nil, err
	}

	childSnapshot := childState.Finalize()

	err = txnState.current().Merge(childSnapshot)
	if err != nil {
		return nil, err
	}

	return childSnapshot, nil
}

func (txnState *transactionState) CommitNestedTransaction(
	expectedId NestedTransactionId,
) (
	*snapshot.ExecutionSnapshot,
	error,
) {
	if !txnState.IsCurrent(expectedId) {
		return nil, fmt.Errorf(
			"cannot commit unexpected nested transaction: id mismatch",
		)
	}

	if txnState.IsParseRestricted() {
		// This is due to a programming error.
		return nil, fmt.Errorf(
			"cannot commit unexpected nested transaction: parse restricted",
		)
	}

	return txnState.mergeIntoParent()
}

func (txnState *transactionState) CommitParseRestrictedNestedTransaction(
	location common.AddressLocation,
) (
	*snapshot.ExecutionSnapshot,
	error,
) {
	currentFrame := txnState.current()
	if currentFrame.parseRestriction == nil ||
		*currentFrame.parseRestriction != location {

		// This is due to a programming error.
		return nil, fmt.Errorf(
			"cannot commit unexpected nested transaction %v != %v",
			currentFrame.parseRestriction,
			location,
		)
	}

	return txnState.mergeIntoParent()
}

func (txnState *transactionState) AttachAndCommitNestedTransaction(
	cachedSnapshot *snapshot.ExecutionSnapshot,
) error {
	return txnState.current().Merge(cachedSnapshot)
}

func (txnState *transactionState) RestartNestedTransaction(
	id NestedTransactionId,
) error {

	// NOTE: We need to verify the id is valid before any merge operation or
	// else we would accidently merge everything into the main transaction.
	found := false
	for _, frame := range txnState.nestedTransactions {
		if frame.ExecutionState == id.state {
			found = true
			break
		}
	}

	if !found {
		return fmt.Errorf(
			"cannot restart nested transaction: nested transaction not found")
	}

	for txnState.current().ExecutionState != id.state {
		_, err := txnState.mergeIntoParent()
		if err != nil {
			return fmt.Errorf("cannot restart nested transaction: %w", err)
		}
	}

	return txnState.current().DropChanges()
}

func (txnState *transactionState) Get(
	id flow.RegisterID,
) (
	flow.RegisterValue,
	error,
) {
	return txnState.current().Get(id)
}

func (txnState *transactionState) Set(
	id flow.RegisterID,
	value flow.RegisterValue,
) error {
	return txnState.current().Set(id, value)
}

func (txnState *transactionState) MeterComputation(
	kind common.ComputationKind,
	intensity uint,
) error {
	return txnState.current().MeterComputation(kind, intensity)
}

func (txnState *transactionState) ComputationAvailable(
	kind common.ComputationKind,
	intensity uint,
) bool {
	return txnState.current().ComputationAvailable(kind, intensity)
}

func (txnState *transactionState) MeterMemory(
	kind common.MemoryKind,
	intensity uint,
) error {
	return txnState.current().MeterMemory(kind, intensity)
}

func (txnState *transactionState) ComputationIntensities() meter.MeteredComputationIntensities {
	return txnState.current().ComputationIntensities()
}

func (txnState *transactionState) TotalComputationLimit() uint {
	return txnState.current().TotalComputationLimit()
}

func (txnState *transactionState) TotalComputationUsed() uint64 {
	return txnState.current().TotalComputationUsed()
}

func (txnState *transactionState) MemoryIntensities() meter.MeteredMemoryIntensities {
	return txnState.current().MemoryIntensities()
}

func (txnState *transactionState) TotalMemoryEstimate() uint64 {
	return txnState.current().TotalMemoryEstimate()
}

func (txnState *transactionState) InteractionUsed() uint64 {
	return txnState.current().InteractionUsed()
}

func (txnState *transactionState) MeterEmittedEvent(byteSize uint64) error {
	return txnState.current().MeterEmittedEvent(byteSize)
}

func (txnState *transactionState) TotalEmittedEventBytes() uint64 {
	return txnState.current().TotalEmittedEventBytes()
}

func (txnState *transactionState) RunWithAllLimitsDisabled(f func()) {
	txnState.current().RunWithAllLimitsDisabled(f)
}
