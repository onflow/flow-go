package state

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"sort"

	"github.com/onflow/cadence/runtime/common"

	"github.com/onflow/flow-go/fvm/errors"
	"github.com/onflow/flow-go/fvm/meter"
	"github.com/onflow/flow-go/model/flow"
)

const (
	DefaultMaxKeySize         = 16_000      // ~16KB
	DefaultMaxValueSize       = 256_000_000 // ~256MB
	DefaultMaxInteractionSize = 20_000_000  // ~20MB

	AccountKeyPrefix = "a."
	KeyAccountStatus = AccountKeyPrefix + "s"
	KeyCode          = "code"
	KeyContractNames = "contract_names"
)

// State represents the execution state
// it holds draft of updates and captures
// all register touches
type State struct {
	view             View
	meter            *meter.Meter
	updatedAddresses map[flow.Address]struct{}
	stateLimits
}

type stateLimits struct {
	maxKeySizeAllowed   uint64
	maxValueSizeAllowed uint64
}

type StateParameters struct {
	meter.MeterParameters

	stateLimits
}

func DefaultParameters() StateParameters {
	return StateParameters{
		MeterParameters: meter.DefaultParameters(),
		stateLimits: stateLimits{
			maxKeySizeAllowed:   DefaultMaxKeySize,
			maxValueSizeAllowed: DefaultMaxValueSize,
		},
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

// WithMaxInteractionSizeAllowed sets limit on total byte interaction with ledger
func (params StateParameters) WithMaxInteractionSizeAllowed(limit uint64) StateParameters {
	newParams := params
	newParams.MeterParameters = newParams.MeterParameters.WithStorageInteractionLimit(limit)
	return newParams
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
		view:             view,
		meter:            m,
		updatedAddresses: make(map[flow.Address]struct{}),
		stateLimits:      params.stateLimits,
	}
}

// NewChild generates a new child state
func (s *State) NewChild() *State {
	return &State{
		view:             s.view.NewChild(),
		meter:            s.meter.NewChild(),
		updatedAddresses: make(map[flow.Address]struct{}),
		stateLimits:      s.stateLimits,
	}
}

// InteractionUsed returns the amount of ledger interaction (total ledger byte read + total ledger byte written)
func (s *State) InteractionUsed() uint64 {
	return s.meter.TotalBytesOfStorageInteractions()
}

// Get returns a register value given owner and key
func (s *State) Get(owner, key string, enforceLimit bool) (flow.RegisterValue, error) {
	var value []byte
	var err error

	if enforceLimit {
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
		meter.StorageInteractionKey{Owner: owner, Key: key},
		value,
		enforceLimit)

	return value, err
}

// Set updates state delta with a register update
func (s *State) Set(owner, key string, value flow.RegisterValue, enforceLimit bool) error {
	if enforceLimit {
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
		meter.StorageInteractionKey{Owner: owner, Key: key},
		value,
		enforceLimit,
	)
	if err != nil {
		return err
	}

	if address, isAddress := addressFromOwner(owner); isAddress {
		s.updatedAddresses[address] = struct{}{}
	}

	return nil
}

// Delete deletes a register
func (s *State) Delete(owner, key string, enforceLimit bool) error {
	return s.Set(owner, key, nil, enforceLimit)
}

// Touch touches a register
func (s *State) Touch(owner, key string) error {
	return s.view.Touch(owner, key)
}

// MeterComputation meters computation usage
func (s *State) MeterComputation(kind common.ComputationKind, intensity uint) error {
	return s.meter.MeterComputation(kind, intensity)
}

// TotalComputationUsed returns total computation used
func (s *State) TotalComputationUsed() uint {
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
	return s.meter.MeterMemory(kind, intensity)
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
	return s.meter.MeterEmittedEvent(byteSize)
}

func (s *State) TotalEmittedEventBytes() uint64 {
	return s.meter.TotalEmittedEventBytes()
}

// MergeState applies the changes from a the given view to this view.
func (s *State) MergeState(other *State) error {
	err := s.view.MergeView(other.view)
	if err != nil {
		return errors.NewStateMergeFailure(err)
	}

	s.meter.MergeMeter(other.meter)

	// apply address updates
	for k, v := range other.updatedAddresses {
		s.updatedAddresses[k] = v
	}

	return nil
}

type sortedAddresses []flow.Address

func (a sortedAddresses) Len() int           { return len(a) }
func (a sortedAddresses) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a sortedAddresses) Less(i, j int) bool { return bytes.Compare(a[i][:], a[j][:]) >= 0 }

// UpdatedAddresses returns a sorted list of addresses that were updated (at least 1 register update)
func (s *State) UpdatedAddresses() []flow.Address {
	addresses := make(sortedAddresses, len(s.updatedAddresses))

	i := 0
	for k := range s.updatedAddresses {
		addresses[i] = k
		i++
	}

	sort.Sort(addresses)

	return addresses
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

func addressFromOwner(owner string) (flow.Address, bool) {
	ownerBytes := []byte(owner)
	if len(ownerBytes) != flow.AddressLength {
		// not an address
		return flow.EmptyAddress, false
	}
	address := flow.BytesToAddress(ownerBytes)
	return address, true
}

// IsFVMStateKey returns true if the
// key is controlled by the fvm env and
// return false otherwise (key controlled by the cadence env)
func IsFVMStateKey(owner, key string) bool {

	// check if is a service level key (owner is empty)
	// cases:
	// 		- "", "uuid"
	// 		- "", "account_address_state"
	if len(owner) == 0 {
		return true
	}
	// check account level keys
	// cases:
	// 		- address, "contract_names"
	// 		- address, "code.%s" (contract name)
	// 		- address, "public_key_%d" (index)
	// 		- address, "a.s" (account status)

	if bytes.HasPrefix([]byte(key), []byte("public_key_")) {
		return true
	}
	if key == KeyContractNames {
		return true
	}
	if bytes.HasPrefix([]byte(key), []byte(KeyCode)) {
		return true
	}
	if key == KeyAccountStatus {
		return true
	}

	return false
}

// PrintableKey formats slabs properly and avoids invalid utf8s
func PrintableKey(key string) string {
	// slab
	if key[0] == '$' && len(key) == 9 {
		i := uint64(binary.BigEndian.Uint64([]byte(key[1:])))
		return fmt.Sprintf("$%d", i)
	}
	return fmt.Sprintf("#%x", []byte(key))
}
