package state

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/onflow/cadence/runtime/common"

	"github.com/onflow/flow-go/fvm/errors"
	"github.com/onflow/flow-go/fvm/meter"
	"github.com/onflow/flow-go/model/flow"
)

const (
	DefaultMaxKeySize   = 16_000      // ~16KB
	DefaultMaxValueSize = 256_000_000 // ~256MB

	// Service level keys (owner is empty):
	UUIDKey         = "uuid"
	AddressStateKey = "account_address_state"

	// Account level keys
	AccountKeyPrefix   = "a."
	AccountStatusKey   = AccountKeyPrefix + "s"
	CodeKeyPrefix      = "code."
	ContractNamesKey   = "contract_names"
	PublicKeyKeyPrefix = "public_key_"
)

// TODO(patrick): make State implement the View interface.
//
// State represents the execution state
// it holds draft of updates and captures
// all register touches
type State struct {
	// NOTE: A committed state is no longer accessible.  It can however be
	// re-attached to another transaction and be committed (for cached result
	// bookkeeping purpose).
	committed bool

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
func (params StateParameters) WithMaxKeySizeAllowed(limit uint64) StateParameters {
	newParams := params
	newParams.maxKeySizeAllowed = limit
	return newParams
}

// WithMaxValueSizeAllowed sets limit on max value size
func (params StateParameters) WithMaxValueSizeAllowed(limit uint64) StateParameters {
	newParams := params
	newParams.maxValueSizeAllowed = limit
	return newParams
}

// TODO(patrick): rm once https://github.com/onflow/flow-emulator/pull/245
// is integrated.
//
// WithMaxInteractionSizeAllowed sets limit on total byte interaction with ledger
func (params StateParameters) WithMaxInteractionSizeAllowed(
	limit uint64,
) StateParameters {
	newParams := params
	newParams.MeterParameters = newParams.MeterParameters.
		WithStorageInteractionLimit(limit)
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

func (s *State) Meter() *meter.Meter {
	return s.meter
}

type StateOption func(st *State) *State

// NewState constructs a new state
func NewState(view View, params StateParameters) *State {
	m := meter.NewMeter(params.MeterParameters)
	return &State{
		committed:        false,
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
		committed:        false,
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

// UpdatedRegisterIDs returns the lists of register ids that were updated.
func (s *State) UpdatedRegisterIDs() []flow.RegisterID {
	return s.view.UpdatedRegisterIDs()
}

// UpdatedRegisters returns the lists of register entries that were updated.
func (s *State) UpdatedRegisters() flow.RegisterEntries {
	return s.view.UpdatedRegisters()
}

// Get returns a register value given owner and key
func (s *State) Get(owner, key string) (flow.RegisterValue, error) {
	if s.committed {
		return nil, fmt.Errorf("cannot Get on a committed state")
	}

	var value []byte
	var err error

	if s.enforceLimits {
		if err = s.checkSize(owner, key, []byte{}); err != nil {
			return nil, err
		}
	}

	if value, err = s.view.Get(owner, key); err != nil {
		// wrap error into a fatal error
		getError := errors.NewLedgerFailure(err)
		// wrap with more info
		return nil, fmt.Errorf("failed to read key %s on account %s: %w", PrintableKey(key), hex.EncodeToString([]byte(owner)), getError)
	}

	err = s.meter.MeterStorageRead(
		flow.RegisterID{Owner: owner, Key: key},
		value,
		s.enforceLimits)

	return value, err
}

// Set updates state delta with a register update
func (s *State) Set(owner, key string, value flow.RegisterValue) error {
	if s.committed {
		return fmt.Errorf("cannot Set on a committed state")
	}

	if s.enforceLimits {
		if err := s.checkSize(owner, key, value); err != nil {
			return err
		}
	}

	if err := s.view.Set(owner, key, value); err != nil {
		// wrap error into a fatal error
		setError := errors.NewLedgerFailure(err)
		// wrap with more info
		return fmt.Errorf("failed to update key %s on account %s: %w", PrintableKey(key), hex.EncodeToString([]byte(owner)), setError)
	}

	err := s.meter.MeterStorageWrite(
		flow.RegisterID{Owner: owner, Key: key},
		value,
		s.enforceLimits,
	)
	if err != nil {
		return err
	}

	return nil
}

// MeterComputation meters computation usage
func (s *State) MeterComputation(kind common.ComputationKind, intensity uint) error {
	if s.committed {
		return fmt.Errorf("cannot MeterComputation on a committed state")
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
	if s.committed {
		return fmt.Errorf("cannot MeterMemory on a committed state")
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
	if s.committed {
		return fmt.Errorf("cannot MeterEmittedEvent on a committed state")
	}

	if s.enforceLimits {
		return s.meter.MeterEmittedEvent(byteSize)
	}

	return nil
}

func (s *State) TotalEmittedEventBytes() uint64 {
	return s.meter.TotalEmittedEventBytes()
}

// MergeState applies the changes from a the given view to this view.
func (s *State) MergeState(other *State) error {
	if s.committed {
		return fmt.Errorf("cannot MergeState on a committed state")
	}

	err := s.view.MergeView(other.view)
	if err != nil {
		return errors.NewStateMergeFailure(err)
	}

	s.meter.MergeMeter(other.meter)

	return nil
}

func (s *State) checkSize(owner, key string, value flow.RegisterValue) error {
	keySize := uint64(len(owner) + len(key))
	valueSize := uint64(len(value))
	if keySize > s.maxKeySizeAllowed {
		return errors.NewStateKeySizeLimitError(owner, key, keySize, s.maxKeySizeAllowed)
	}
	if valueSize > s.maxValueSizeAllowed {
		return errors.NewStateValueSizeLimitError(value, valueSize, s.maxValueSizeAllowed)
	}
	return nil
}

// IsFVMStateKey returns true if the key is controlled by the fvm env and
// return false otherwise (key controlled by the cadence env)
func IsFVMStateKey(owner string, key string) bool {
	// check if is a service level key (owner is empty)
	// cases:
	// 		- "", "uuid"
	// 		- "", "account_address_state"
	if len(owner) == 0 && (key == UUIDKey || key == AddressStateKey) {
		return true
	}

	// check account level keys
	// cases:
	// 		- address, "contract_names"
	// 		- address, "code.%s" (contract name)
	// 		- address, "public_key_%d" (index)
	// 		- address, "a.s" (account status)
	return strings.HasPrefix(key, PublicKeyKeyPrefix) ||
		key == ContractNamesKey ||
		strings.HasPrefix(key, CodeKeyPrefix) ||
		key == AccountStatusKey
}

// This returns true if the key is a slab index for an account's ordered fields
// map.
//
// In general, each account's regular fields are stored in ordered map known
// only to cadence.  Cadence encodes this map into bytes and split the bytes
// into slab chunks before storing the slabs into the ledger.
func IsSlabIndex(key string) bool {
	return len(key) == 9 && key[0] == '$'
}

// PrintableKey formats slabs properly and avoids invalid utf8s
func PrintableKey(key string) string {
	// slab
	if IsSlabIndex(key) {
		i := uint64(binary.BigEndian.Uint64([]byte(key[1:])))
		return fmt.Sprintf("$%d", i)
	}
	return fmt.Sprintf("#%x", []byte(key))
}
