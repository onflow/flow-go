package state

import (
	"fmt"
	"math"

	"github.com/onflow/cadence/common"
	"github.com/onflow/crypto/hash"

	"github.com/onflow/flow-go/fvm/errors"
	"github.com/onflow/flow-go/fvm/meter"
	"github.com/onflow/flow-go/fvm/storage/snapshot"
	"github.com/onflow/flow-go/model/flow"
)

const (
	DefaultMaxKeySize   = 16_000      // ~16KB
	DefaultMaxValueSize = 256_000_000 // ~256MB
)

// State represents the execution state
// it holds draft of updates and captures
// all register touches
type ExecutionState struct {
	// NOTE: A finalized state is no longer accessible.  It can however be
	// re-attached to another transaction and be committed (for cached result
	// bookkeeping purpose).
	finalized bool

	*spockState
	meter *meter.Meter

	// NOTE: parent and child state shares the same limits controller
	*limitsController
}

type StateParameters struct {
	meter.MeterParameters

	maxKeySizeAllowed   uint64
	maxValueSizeAllowed uint64
}

type ExecutionParameters struct {
	meter.MeterParameters
}

func DefaultParameters() StateParameters {
	return StateParameters{
		MeterParameters:     meter.DefaultParameters(),
		maxKeySizeAllowed:   DefaultMaxKeySize,
		maxValueSizeAllowed: DefaultMaxValueSize,
	}
}

// WithMeterParameters sets the state's meter parameters
func (params StateParameters) WithMeterParameters(
	meterParams meter.MeterParameters,
) StateParameters {
	newParams := params
	newParams.MeterParameters = meterParams
	return newParams
}

// WithMaxKeySizeAllowed sets limit on max key size
func (params StateParameters) WithMaxKeySizeAllowed(
	limit uint64,
) StateParameters {
	newParams := params
	newParams.maxKeySizeAllowed = limit
	return newParams
}

// WithMaxValueSizeAllowed sets limit on max value size
func (params StateParameters) WithMaxValueSizeAllowed(
	limit uint64,
) StateParameters {
	newParams := params
	newParams.maxValueSizeAllowed = limit
	return newParams
}

type limitsController struct {
	meteringEnabled     bool
	maxKeySizeAllowed   uint64
	maxValueSizeAllowed uint64
}

func newLimitsController(params StateParameters) *limitsController {
	return &limitsController{
		meteringEnabled:     true,
		maxKeySizeAllowed:   params.maxKeySizeAllowed,
		maxValueSizeAllowed: params.maxValueSizeAllowed,
	}
}

// RunWithMeteringDisabled runs f with metering disabled. While metering is
// disabled, none of the metered quantities (computation, memory, events, and
// ledger interaction) are accumulated or limited. The previous metering state
// is restored afterwards, so nested calls behave correctly.
func (controller *limitsController) RunWithMeteringDisabled(f func()) {
	if f == nil {
		return
	}
	current := controller.meteringEnabled
	controller.meteringEnabled = false
	f()
	controller.meteringEnabled = current
}

// NewExecutionState constructs a new state
func NewExecutionState(
	snapshot snapshot.StorageSnapshot,
	params StateParameters,
) *ExecutionState {
	return NewExecutionStateWithSpockStateHasher(
		snapshot,
		params,
		DefaultSpockSecretHasher,
	)
}

// NewExecutionStateWithSpockStateHasher constructs a new state with a custom hasher
func NewExecutionStateWithSpockStateHasher(
	snapshot snapshot.StorageSnapshot,
	params StateParameters,
	getHasher func() hash.Hasher,
) *ExecutionState {
	m := meter.NewMeter(params.MeterParameters)
	return &ExecutionState{
		finalized:        false,
		spockState:       newSpockState(snapshot, getHasher),
		meter:            m,
		limitsController: newLimitsController(params),
	}
}

// NewChildWithMeterParams generates a new child state using the provide meter
// parameters.
func (state *ExecutionState) NewChildWithMeterParams(
	params ExecutionParameters,
) *ExecutionState {
	return &ExecutionState{
		finalized:        false,
		spockState:       state.spockState.NewChild(),
		meter:            meter.NewMeter(params.MeterParameters),
		limitsController: state.limitsController,
	}
}

// NewChild generates a new child state using the parent's meter parameters.
func (state *ExecutionState) NewChild() *ExecutionState {
	return state.NewChildWithMeterParams(state.ExecutionParameters())
}

// NewChildForDerivedData generates a new child state for computing a derived
// data value (e.g. loading a program into the programs cache). The child
// meters unconditionally, even when the parent runs with metering disabled.
//
// Unlike NewChild, the child does NOT share the parent's limitsController: it
// gets a fresh controller with meteringEnabled=true. This is used by the
// derived data cache so that a value loaded into the cache always carries a
// fully-populated meter, independent of the caller's metering scope (see
// internal issue #7126). Whether those charges are applied to the caller is
// decided when the child is merged back, by Merge gating on the caller's
// metering.
//
// When the parent's metering is disabled, the child's meter limits are lifted
// (weights unchanged) so that loads performed in system-critical
// metering-disabled scopes (e.g. fee deduction) can never fail on a limit,
// while still recording deterministic intensities.
func (state *ExecutionState) NewChildForDerivedData() *ExecutionState {
	params := state.ExecutionParameters()
	if !state.meteringEnabled {
		params.MeterParameters = params.MeterParameters.WithoutLimits()
	}

	return &ExecutionState{
		finalized:  false,
		spockState: state.spockState.NewChild(),
		meter:      meter.NewMeter(params.MeterParameters),
		limitsController: &limitsController{
			meteringEnabled:     true,
			maxKeySizeAllowed:   state.maxKeySizeAllowed,
			maxValueSizeAllowed: state.maxValueSizeAllowed,
		},
	}
}

// InteractionUsed returns the amount of ledger interaction (total ledger byte read + total ledger byte written)
func (state *ExecutionState) InteractionUsed() uint64 {
	return state.meter.TotalBytesOfStorageInteractions()
}

// BytesWritten returns the amount of total ledger bytes written
func (state *ExecutionState) BytesWritten() uint64 {
	return state.meter.TotalBytesWrittenToStorage()
}

func (state *ExecutionState) DropChanges() error {
	if state.finalized {
		return fmt.Errorf("cannot DropChanges on a finalized state")
	}

	return state.spockState.DropChanges()
}

// Get returns a register value given owner and key. Storage interaction is only
// metered (accumulated and limited) when metering is enabled; when metering is
// disabled the read is neither counted nor limited.
//
// Expected error returns during normal operation:
//   - [errors.StateKeySizeLimitError] if the key exceeds the key size limit
//   - [errors.LimitExceededError] with [errors.LimitKindLedgerInteraction] if
//     the storage interaction limit is exceeded
//
// All other errors are exceptions (ledger failures, use after finalization).
func (state *ExecutionState) Get(id flow.RegisterID) (flow.RegisterValue, error) {
	if state.finalized {
		return nil, fmt.Errorf("cannot Get on a finalized state")
	}

	var value []byte
	var err error

	if state.meteringEnabled {
		if err = state.checkSize(id, []byte{}); err != nil {
			return nil, err
		}
	}

	if value, err = state.spockState.Get(id); err != nil {
		// wrap error into a fatal error
		getError := errors.NewLedgerFailure(err)
		// wrap with more info
		return nil, fmt.Errorf("failed to read %s: %w", id, getError)
	}

	if state.meteringEnabled {
		if err = state.meter.MeterStorageRead(id, value); err != nil {
			return value, err
		}
	}
	return value, nil
}

// Set updates state delta with a register update. Storage interaction is only
// metered (accumulated and limited) when metering is enabled; when metering is
// disabled the write is neither counted nor limited.
//
// Expected error returns during normal operation:
//   - [errors.StateKeySizeLimitError] or [errors.StateValueSizeLimitError] if
//     the key or value exceeds its size limit
//   - [errors.LimitExceededError] with [errors.LimitKindLedgerInteraction] if
//     the storage interaction limit is exceeded
//
// All other errors are exceptions (ledger failures, use after finalization).
func (state *ExecutionState) Set(id flow.RegisterID, value flow.RegisterValue) error {
	if state.finalized {
		return fmt.Errorf("cannot Set on a finalized state")
	}

	if state.meteringEnabled {
		if err := state.checkSize(id, value); err != nil {
			return err
		}
	}

	if err := state.spockState.Set(id, value); err != nil {
		// wrap error into a fatal error
		setError := errors.NewLedgerFailure(err)
		// wrap with more info
		return fmt.Errorf("failed to update %s: %w", id, setError)
	}

	if state.meteringEnabled {
		return state.meter.MeterStorageWrite(id, value)
	}
	return nil
}

// MeterComputation meters computation usage. It is a no-op if metering is
// disabled.
//
// Expected error returns during normal operation:
//   - [errors.LimitExceededError] with [errors.LimitKindComputation] if the
//     computation limit is exceeded
//
// Returns an exception if called on a finalized state.
func (state *ExecutionState) MeterComputation(usage common.ComputationUsage) error {
	if state.finalized {
		return fmt.Errorf("cannot MeterComputation on a finalized state")
	}

	if state.meteringEnabled {
		return state.meter.MeterComputation(usage)
	}
	return nil
}

// ComputationRemaining returns the remaining computation for the given kind.
func (state *ExecutionState) ComputationRemaining(kind common.ComputationKind) uint64 {
	if state.finalized {
		// if state is finalized return false
		return 0
	}

	if state.meteringEnabled {
		return state.meter.ComputationRemaining(kind)
	}
	return math.MaxUint64
}

// TotalComputationUsed returns total computation used
func (state *ExecutionState) TotalComputationUsed() uint64 {
	return state.meter.TotalComputationUsed()
}

// ComputationIntensities returns computation intensities
func (state *ExecutionState) ComputationIntensities() meter.MeteredComputationIntensities {
	return state.meter.ComputationIntensities()
}

// TotalComputationLimit returns total computation limit
func (state *ExecutionState) TotalComputationLimit() uint64 {
	return state.meter.TotalComputationLimit()
}

// MeterMemory meters memory usage. It is a no-op if metering is disabled.
//
// Expected error returns during normal operation:
//   - [errors.LimitExceededError] with [errors.LimitKindMemory] if the memory
//     limit is exceeded
//
// Returns an exception if called on a finalized state.
func (state *ExecutionState) MeterMemory(usage common.MemoryUsage) error {
	if state.finalized {
		return fmt.Errorf("cannot MeterMemory on a finalized state")
	}

	if state.meteringEnabled {
		return state.meter.MeterMemory(usage)
	}

	return nil
}

// MemoryAmounts returns memory amounts
func (state *ExecutionState) MemoryAmounts() meter.MeteredMemoryAmounts {
	return state.meter.MemoryAmounts()
}

// TotalMemoryEstimate returns total memory used
func (state *ExecutionState) TotalMemoryEstimate() uint64 {
	return state.meter.TotalMemoryEstimate()
}

// TotalMemoryLimit returns total memory limit
func (state *ExecutionState) TotalMemoryLimit() uint {
	return uint(state.meter.TotalMemoryLimit())
}

// MeterEmittedEvent captures the byte size of an emitted event. It is a no-op
// if metering is disabled.
//
// Expected error returns during normal operation:
//   - [errors.LimitExceededError] with [errors.LimitKindEvent] if the event
//     byte size limit is exceeded
//
// Returns an exception if called on a finalized state.
func (state *ExecutionState) MeterEmittedEvent(byteSize uint64) error {
	if state.finalized {
		return fmt.Errorf("cannot MeterEmittedEvent on a finalized state")
	}

	if state.meteringEnabled {
		return state.meter.MeterEmittedEvent(byteSize)
	}

	return nil
}

func (state *ExecutionState) Finalize() *snapshot.ExecutionSnapshot {
	state.finalized = true
	snapshot := state.spockState.Finalize()
	snapshot.Meter = state.meter
	return snapshot
}

// Merge applies the changes from the given execution snapshot to this state.
// The read/write set and SPoCK are always merged. The snapshot's meter
// (computation, memory, events, and ledger interaction) is merged only when
// this state's metering is enabled, so charges from a nested transaction are
// applied to the caller only while the caller is metering. This keeps program
// cache metering deterministic: a value loaded into the cache is always fully
// metered (see NewChildForDerivedData), but replaying it (or a fresh load)
// charges the caller only when metering is enabled (internal issue #7126).
func (state *ExecutionState) Merge(other *snapshot.ExecutionSnapshot) error {
	if state.finalized {
		return fmt.Errorf("cannot Merge on a finalized state")
	}

	err := state.spockState.Merge(other)
	if err != nil {
		return errors.NewStateMergeFailure(err)
	}

	if state.meteringEnabled {
		state.meter.MergeMeter(other.Meter)
	}
	return nil
}

func (state *ExecutionState) checkSize(
	id flow.RegisterID,
	value flow.RegisterValue,
) error {
	keySize := uint64(len(id.Owner) + len(id.Key))
	valueSize := uint64(len(value))
	if keySize > state.maxKeySizeAllowed {
		return errors.NewStateKeySizeLimitError(
			id,
			keySize,
			state.maxKeySizeAllowed)
	}
	if valueSize > state.maxValueSizeAllowed {
		return errors.NewStateValueSizeLimitError(
			value,
			valueSize,
			state.maxValueSizeAllowed)
	}
	return nil
}

func (state *ExecutionState) ExecutionParameters() ExecutionParameters {
	return ExecutionParameters{
		MeterParameters: state.meter.MeterParameters,
	}
}

func (state *ExecutionState) readSetSize() int {
	return state.spockState.readSetSize()
}

func (state *ExecutionState) interimReadSet(
	accumulator map[flow.RegisterID]struct{},
) {
	state.spockState.interimReadSet(accumulator)
}
