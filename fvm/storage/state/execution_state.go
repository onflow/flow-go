package state

import (
	"fmt"

	"github.com/onflow/cadence/runtime/common"

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
	enforceLimits       bool
	maxKeySizeAllowed   uint64
	maxValueSizeAllowed uint64
}

func newLimitsController(params StateParameters) *limitsController {
	return &limitsController{
		enforceLimits:       true,
		maxKeySizeAllowed:   params.maxKeySizeAllowed,
		maxValueSizeAllowed: params.maxValueSizeAllowed,
	}
}

func (controller *limitsController) RunWithAllLimitsDisabled(f func()) {
	if f == nil {
		return
	}
	current := controller.enforceLimits
	controller.enforceLimits = false
	f()
	controller.enforceLimits = current
}

// NewExecutionState constructs a new state
func NewExecutionState(
	snapshot snapshot.StorageSnapshot,
	params StateParameters,
) *ExecutionState {
	m := meter.NewMeter(params.MeterParameters)
	return &ExecutionState{
		finalized:        false,
		spockState:       newSpockState(snapshot),
		meter:            m,
		limitsController: newLimitsController(params),
	}
}

// NewChildWithMeterParams generates a new child state using the provide meter
// parameters.
func (state *ExecutionState) NewChildWithMeterParams(
	params meter.MeterParameters,
) *ExecutionState {
	return &ExecutionState{
		finalized:        false,
		spockState:       state.spockState.NewChild(),
		meter:            meter.NewMeter(params),
		limitsController: state.limitsController,
	}
}

// NewChild generates a new child state using the parent's meter parameters.
func (state *ExecutionState) NewChild() *ExecutionState {
	return state.NewChildWithMeterParams(state.meter.MeterParameters)
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

// Get returns a register value given owner and key
func (state *ExecutionState) Get(id flow.RegisterID) (flow.RegisterValue, error) {
	if state.finalized {
		return nil, fmt.Errorf("cannot Get on a finalized state")
	}

	var value []byte
	var err error

	if state.enforceLimits {
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

	err = state.meter.MeterStorageRead(id, value, state.enforceLimits)
	return value, err
}

// Set updates state delta with a register update
func (state *ExecutionState) Set(id flow.RegisterID, value flow.RegisterValue) error {
	if state.finalized {
		return fmt.Errorf("cannot Set on a finalized state")
	}

	if state.enforceLimits {
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

	return state.meter.MeterStorageWrite(id, value, state.enforceLimits)
}

// MeterComputation meters computation usage
func (state *ExecutionState) MeterComputation(kind common.ComputationKind, intensity uint) error {
	if state.finalized {
		return fmt.Errorf("cannot MeterComputation on a finalized state")
	}

	if state.enforceLimits {
		return state.meter.MeterComputation(kind, intensity)
	}
	return nil
}

// ComputationAvailable checks if enough computation capacity is available without metering
func (state *ExecutionState) ComputationAvailable(kind common.ComputationKind, intensity uint) bool {
	if state.finalized {
		// if state is finalized return false
		return false
	}

	if state.enforceLimits {
		return state.meter.ComputationAvailable(kind, intensity)
	}
	return true
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
func (state *ExecutionState) TotalComputationLimit() uint {
	return state.meter.TotalComputationLimit()
}

// MeterMemory meters memory usage
func (state *ExecutionState) MeterMemory(kind common.MemoryKind, intensity uint) error {
	if state.finalized {
		return fmt.Errorf("cannot MeterMemory on a finalized state")
	}

	if state.enforceLimits {
		return state.meter.MeterMemory(kind, intensity)
	}

	return nil
}

// MemoryIntensities returns computation intensities
func (state *ExecutionState) MemoryIntensities() meter.MeteredMemoryIntensities {
	return state.meter.MemoryIntensities()
}

// TotalMemoryEstimate returns total memory used
func (state *ExecutionState) TotalMemoryEstimate() uint64 {
	return state.meter.TotalMemoryEstimate()
}

// TotalMemoryLimit returns total memory limit
func (state *ExecutionState) TotalMemoryLimit() uint {
	return uint(state.meter.TotalMemoryLimit())
}

func (state *ExecutionState) MeterEmittedEvent(byteSize uint64) error {
	if state.finalized {
		return fmt.Errorf("cannot MeterEmittedEvent on a finalized state")
	}

	if state.enforceLimits {
		return state.meter.MeterEmittedEvent(byteSize)
	}

	return nil
}

func (state *ExecutionState) TotalEmittedEventBytes() uint64 {
	return state.meter.TotalEmittedEventBytes()
}

func (state *ExecutionState) Finalize() *snapshot.ExecutionSnapshot {
	state.finalized = true
	snapshot := state.spockState.Finalize()
	snapshot.Meter = state.meter
	return snapshot
}

// MergeState the changes from a the given execution snapshot to this state.
func (state *ExecutionState) Merge(other *snapshot.ExecutionSnapshot) error {
	if state.finalized {
		return fmt.Errorf("cannot Merge on a finalized state")
	}

	err := state.spockState.Merge(other)
	if err != nil {
		return errors.NewStateMergeFailure(err)
	}

	state.meter.MergeMeter(other.Meter)
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

func (state *ExecutionState) readSetSize() int {
	return state.spockState.readSetSize()
}

func (state *ExecutionState) interimReadSet(
	accumulator map[flow.RegisterID]struct{},
) {
	state.spockState.interimReadSet(accumulator)
}
