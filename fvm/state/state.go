package state

import (
	"fmt"

	"github.com/onflow/cadence/runtime/common"

	"github.com/onflow/flow-go/fvm/errors"
	"github.com/onflow/flow-go/fvm/meter"
	"github.com/onflow/flow-go/model/flow"
)

const (
	DefaultMaxKeySize   = 16_000      // ~16KB
	DefaultMaxValueSize = 256_000_000 // ~256MB
)

// TODO(patrick): make State implement the View interface.
//
// State represents the execution state
// it holds draft of updates and captures
// all register touches
type State struct {
	// NOTE: A finalized view is no longer accessible.  It can however be
	// re-attached to another transaction and be committed (for cached result
	// bookkeeping purpose).
	finalized bool

	view  View
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

func (s *State) View() View {
	return s.view
}

// NewState constructs a new state
func NewState(view View, params StateParameters) *State {
	m := meter.NewMeter(params.MeterParameters)
	return &State{
		finalized:        false,
		view:             view,
		meter:            m,
		limitsController: newLimitsController(params),
	}
}

// NewChildWithMeterParams generates a new child state using the provide meter
// parameters.
func (s *State) NewChildWithMeterParams(
	params meter.MeterParameters,
) *State {
	return &State{
		finalized:        false,
		view:             s.view.NewChild(),
		meter:            meter.NewMeter(params),
		limitsController: s.limitsController,
	}
}

// NewChild generates a new child state using the parent's meter parameters.
func (s *State) NewChild() *State {
	return s.NewChildWithMeterParams(s.meter.MeterParameters)
}

// InteractionUsed returns the amount of ledger interaction (total ledger byte read + total ledger byte written)
func (s *State) InteractionUsed() uint64 {
	return s.meter.TotalBytesOfStorageInteractions()
}

// BytesWritten returns the amount of total ledger bytes written
func (s *State) BytesWritten() uint64 {
	return s.meter.TotalBytesWrittenToStorage()
}

func (s *State) DropChanges() error {
	if s.finalized {
		return fmt.Errorf("cannot DropChanges on a finalized view")
	}

	return s.view.DropChanges()
}

// Get returns a register value given owner and key
func (s *State) Get(id flow.RegisterID) (flow.RegisterValue, error) {
	if s.finalized {
		return nil, fmt.Errorf("cannot Get on a finalized view")
	}

	var value []byte
	var err error

	if s.enforceLimits {
		if err = s.checkSize(id, []byte{}); err != nil {
			return nil, err
		}
	}

	if value, err = s.view.Get(id); err != nil {
		// wrap error into a fatal error
		getError := errors.NewLedgerFailure(err)
		// wrap with more info
		return nil, fmt.Errorf("failed to read %s: %w", id, getError)
	}

	err = s.meter.MeterStorageRead(id, value, s.enforceLimits)
	return value, err
}

// Set updates state delta with a register update
func (s *State) Set(id flow.RegisterID, value flow.RegisterValue) error {
	if s.finalized {
		return fmt.Errorf("cannot Set on a finalized view")
	}

	if s.enforceLimits {
		if err := s.checkSize(id, value); err != nil {
			return err
		}
	}

	if err := s.view.Set(id, value); err != nil {
		// wrap error into a fatal error
		setError := errors.NewLedgerFailure(err)
		// wrap with more info
		return fmt.Errorf("failed to update %s: %w", id, setError)
	}

	return s.meter.MeterStorageWrite(id, value, s.enforceLimits)
}

// MeterComputation meters computation usage
func (s *State) MeterComputation(kind common.ComputationKind, intensity uint) error {
	if s.finalized {
		return fmt.Errorf("cannot MeterComputation on a finalized view")
	}

	if s.enforceLimits {
		return s.meter.MeterComputation(kind, intensity)
	}
	return nil
}

// TotalComputationUsed returns total computation used
func (s *State) TotalComputationUsed() uint64 {
	return s.meter.TotalComputationUsed()
}

// ComputationIntensities returns computation intensities
func (s *State) ComputationIntensities() meter.MeteredComputationIntensities {
	return s.meter.ComputationIntensities()
}

// TotalComputationLimit returns total computation limit
func (s *State) TotalComputationLimit() uint {
	return s.meter.TotalComputationLimit()
}

// MeterMemory meters memory usage
func (s *State) MeterMemory(kind common.MemoryKind, intensity uint) error {
	if s.finalized {
		return fmt.Errorf("cannot MeterMemory on a finalized view")
	}

	if s.enforceLimits {
		return s.meter.MeterMemory(kind, intensity)
	}

	return nil
}

// MemoryIntensities returns computation intensities
func (s *State) MemoryIntensities() meter.MeteredMemoryIntensities {
	return s.meter.MemoryIntensities()
}

// TotalMemoryEstimate returns total memory used
func (s *State) TotalMemoryEstimate() uint64 {
	return s.meter.TotalMemoryEstimate()
}

// TotalMemoryLimit returns total memory limit
func (s *State) TotalMemoryLimit() uint {
	return uint(s.meter.TotalMemoryLimit())
}

func (s *State) MeterEmittedEvent(byteSize uint64) error {
	if s.finalized {
		return fmt.Errorf("cannot MeterEmittedEvent on a finalized view")
	}

	if s.enforceLimits {
		return s.meter.MeterEmittedEvent(byteSize)
	}

	return nil
}

func (s *State) TotalEmittedEventBytes() uint64 {
	return s.meter.TotalEmittedEventBytes()
}

func (s *State) Finalize() *ExecutionSnapshot {
	s.finalized = true
	snapshot := s.view.Finalize()
	snapshot.Meter = s.meter
	return snapshot
}

// MergeState the changes from a the given view to this view.
func (s *State) Merge(other *ExecutionSnapshot) error {
	if s.finalized {
		return fmt.Errorf("cannot Merge on a finalized view")
	}

	err := s.view.Merge(other)
	if err != nil {
		return errors.NewStateMergeFailure(err)
	}

	s.meter.MergeMeter(other.Meter)
	return nil
}

func (s *State) checkSize(
	id flow.RegisterID,
	value flow.RegisterValue,
) error {
	keySize := uint64(len(id.Owner) + len(id.Key))
	valueSize := uint64(len(value))
	if keySize > s.maxKeySizeAllowed {
		return errors.NewStateKeySizeLimitError(
			id,
			keySize,
			s.maxKeySizeAllowed)
	}
	if valueSize > s.maxValueSizeAllowed {
		return errors.NewStateValueSizeLimitError(
			value,
			valueSize,
			s.maxValueSizeAllowed)
	}
	return nil
}
