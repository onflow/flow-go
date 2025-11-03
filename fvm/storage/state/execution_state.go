package state

import (
	"fmt"

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

type state interface {
	NewChild() state
	Finalize() *snapshot.ExecutionSnapshot
	Merge(snapshot *snapshot.ExecutionSnapshot) error
	Set(id flow.RegisterID, value flow.RegisterValue) error
	Get(id flow.RegisterID) (flow.RegisterValue, error)
	DropChanges() error
	readSetSize() int
	interimReadSet(accumulator map[flow.RegisterID]struct{})
}

// State represents the execution state
// it holds draft of updates and captures
// all register touches
type ExecutionState struct {
	// NOTE: A finalized state is no longer accessible.  It can however be
	// re-attached to another transaction and be committed (for cached result
	// bookkeeping purpose).
	finalized bool

	state
	meter *meter.Meter

	// NOTE: parent and child state shares the same limits controller
	*limitsController
}

var _ state = &ExecutionState{}

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

func (controller *limitsController) RunWithMeteringDisabled(f func()) {
	if f == nil {
		return
	}
	current := controller.meteringEnabled
	controller.meteringEnabled = false
	f()
	controller.meteringEnabled = current
}

// NewSpockExecutionState constructs a new execution state,
// backed by a Spock state and using the default Spock hasher.
func NewSpockExecutionState(
	snapshot snapshot.StorageSnapshot,
	params StateParameters,
) *ExecutionState {
	return NewSpockExecutionStateWithSpockStateHasher(
		snapshot,
		params,
		DefaultSpockSecretHasher,
	)
}

// NewDefaultSpockExecutionState constructs a new execution state,
// backed by a Spock state with default parameters and using the default Spock hasher.
func NewDefaultSpockExecutionState() *ExecutionState {
	return NewSpockExecutionState(nil, DefaultParameters())
}

// NewSpockExecutionStateWithSpockStateHasher constructs a new execution state,
// backed by a Spock state and using a custom Spock hasher.
func NewSpockExecutionStateWithSpockStateHasher(
	snapshot snapshot.StorageSnapshot,
	params StateParameters,
	getHasher func() hash.Hasher,
) *ExecutionState {
	return NewExecutionState(
		newSpockState(snapshot, getHasher),
		params,
	)
}

// Deprecated: NewDefaultWritesOnlySpockExecutionState constructs a new execution state,
// backed by a "writes-only Spock" which only considers writes but not reads,
// with default parameters and using the default Spock hasher.
//
// It should not be used in production code, only for testing and debugging purposes.
func NewDefaultWritesOnlySpockExecutionState() *ExecutionState {
	return NewExecutionState(
		newWritesOnlySpockState(nil, DefaultSpockSecretHasher),
		DefaultParameters(),
	)
}

// NewExecutionState constructs a new execution state backed by the provided state.
func NewExecutionState(state state, params StateParameters) *ExecutionState {
	return &ExecutionState{
		finalized:        false,
		state:            state,
		meter:            meter.NewMeter(params.MeterParameters),
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
		state:            state.state.NewChild(),
		meter:            meter.NewMeter(params.MeterParameters),
		limitsController: state.limitsController,
	}
}

// NewChild generates a new child state using the parent's meter parameters.
func (state *ExecutionState) NewChild() state {
	return state.NewChildWithMeterParams(state.ExecutionParameters())
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

	return state.state.DropChanges()
}

// Get returns a register value given owner and key
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

	if value, err = state.state.Get(id); err != nil {
		// wrap error into a fatal error
		getError := errors.NewLedgerFailure(err)
		// wrap with more info
		return nil, fmt.Errorf("failed to read %s: %w", id, getError)
	}

	err = state.meter.MeterStorageRead(id, value, state.meteringEnabled)
	return value, err
}

// Set updates state delta with a register update
func (state *ExecutionState) Set(id flow.RegisterID, value flow.RegisterValue) error {
	if state.finalized {
		return fmt.Errorf("cannot Set on a finalized state")
	}

	if state.meteringEnabled {
		if err := state.checkSize(id, value); err != nil {
			return err
		}
	}

	if err := state.state.Set(id, value); err != nil {
		// wrap error into a fatal error
		setError := errors.NewLedgerFailure(err)
		// wrap with more info
		return fmt.Errorf("failed to update %s: %w", id, setError)
	}

	return state.meter.MeterStorageWrite(id, value, state.meteringEnabled)
}

// MeterComputation meters computation usage
func (state *ExecutionState) MeterComputation(usage common.ComputationUsage) error {
	if state.finalized {
		return fmt.Errorf("cannot MeterComputation on a finalized state")
	}

	if state.meteringEnabled {
		return state.meter.MeterComputation(usage)
	}
	return nil
}

// ComputationAvailable checks if enough computation capacity is available without metering
func (state *ExecutionState) ComputationAvailable(usage common.ComputationUsage) bool {
	if state.finalized {
		// if state is finalized return false
		return false
	}

	if state.meteringEnabled {
		return state.meter.ComputationAvailable(usage)
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
func (state *ExecutionState) TotalComputationLimit() uint64 {
	return state.meter.TotalComputationLimit()
}

// MeterMemory meters memory usage
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

func (state *ExecutionState) MeterEmittedEvent(byteSize uint64) error {
	if state.finalized {
		return fmt.Errorf("cannot MeterEmittedEvent on a finalized state")
	}

	if state.meteringEnabled {
		return state.meter.MeterEmittedEvent(byteSize)
	}

	return nil
}

func (state *ExecutionState) TotalEmittedEventBytes() uint64 {
	return state.meter.TotalEmittedEventBytes()
}

func (state *ExecutionState) Finalize() *snapshot.ExecutionSnapshot {
	state.finalized = true
	snap := state.state.Finalize()
	snap.Meter = state.meter
	return snap
}

// MergeState the changes from a the given execution snapshot to this state.
func (state *ExecutionState) Merge(other *snapshot.ExecutionSnapshot) error {
	if state.finalized {
		return fmt.Errorf("cannot Merge on a finalized state")
	}

	err := state.state.Merge(other)
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

func (state *ExecutionState) ExecutionParameters() ExecutionParameters {
	return ExecutionParameters{
		MeterParameters: state.meter.MeterParameters,
	}
}

func (state *ExecutionState) readSetSize() int {
	return state.state.readSetSize()
}

func (state *ExecutionState) interimReadSet(
	accumulator map[flow.RegisterID]struct{},
) {
	state.state.interimReadSet(accumulator)
}
