package state

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"sort"

	"github.com/onflow/flow-go/fvm/errors"
	"github.com/onflow/flow-go/fvm/meter"
	"github.com/onflow/flow-go/fvm/meter/noop"
	"github.com/onflow/flow-go/model/flow"
)

const (
	DefaultMaxKeySize         = 16_000      // ~16KB
	DefaultMaxValueSize       = 256_000_000 // ~256MB
	DefaultMaxInteractionSize = 20_000_000  // ~20MB
)

type mapKey struct {
	owner, controller, key string
}

// State represents the execution state
// it holds draft of updates and captures
// all register touches
type State struct {
	view                  View
	meter                 meter.Meter
	updatedAddresses      map[flow.Address]struct{}
	updateSize            map[mapKey]uint64
	maxKeySizeAllowed     uint64
	maxValueSizeAllowed   uint64
	maxInteractionAllowed uint64
	ReadCounter           uint64
	WriteCounter          uint64
	TotalBytesRead        uint64
	TotalBytesWritten     uint64
}

func defaultState(view View) *State {
	return &State{
		view:                  view,
		meter:                 noop.NewMeter(),
		updatedAddresses:      make(map[flow.Address]struct{}),
		updateSize:            make(map[mapKey]uint64),
		maxKeySizeAllowed:     DefaultMaxKeySize,
		maxValueSizeAllowed:   DefaultMaxValueSize,
		maxInteractionAllowed: DefaultMaxInteractionSize,
	}
}

func (s *State) View() View {
	return s.view
}

type StateOption func(st *State) *State

// NewState constructs a new state
func NewState(view View, opts ...StateOption) *State {
	ctx := defaultState(view)
	for _, applyOption := range opts {
		ctx = applyOption(ctx)
	}
	return ctx
}

// WithMaxKeySizeAllowed sets limit on max key size
func WithMaxKeySizeAllowed(limit uint64) func(st *State) *State {
	return func(st *State) *State {
		st.maxKeySizeAllowed = limit
		return st
	}
}

// WithMaxValueSizeAllowed sets limit on max value size
func WithMaxValueSizeAllowed(limit uint64) func(st *State) *State {
	return func(st *State) *State {
		st.maxValueSizeAllowed = limit
		return st
	}
}

// WithMaxInteractionSizeAllowed sets limit on total byte interaction with ledger
func WithMaxInteractionSizeAllowed(limit uint64) func(st *State) *State {
	return func(st *State) *State {
		st.maxInteractionAllowed = limit
		return st
	}
}

// WithMeter sets the meter
func WithMeter(m meter.Meter) func(st *State) *State {
	return func(st *State) *State {
		st.meter = m
		return st
	}
}

// InteractionUsed returns the amount of ledger interaction (total ledger byte read + total ledger byte written)
func (s *State) InteractionUsed() uint64 {
	return s.TotalBytesRead + s.TotalBytesWritten
}

// Get returns a register value given owner, controller and key
func (s *State) Get(owner, controller, key string, enforceLimit bool) (flow.RegisterValue, error) {
	var value []byte
	var err error

	if enforceLimit {
		if err = s.checkSize(owner, controller, key, []byte{}); err != nil {
			return nil, err
		}
	}

	if value, err = s.view.Get(owner, controller, key); err != nil {
		// wrap error into a fatal error
		getError := errors.NewLedgerFailure(err)
		// wrap with more info
		return nil, fmt.Errorf("failed to read key %s on account %s: %w", PrintableKey(key), hex.EncodeToString([]byte(owner)), getError)
	}

	// if not part of recent updates count them as read
	if _, ok := s.updateSize[mapKey{owner, controller, key}]; !ok {
		s.ReadCounter++
		s.TotalBytesRead += uint64(len(owner) +
			len(controller) + len(key) + len(value))
	}

	if enforceLimit {
		return value, s.checkMaxInteraction()
	}

	return value, nil
}

// Set updates state delta with a register update
func (s *State) Set(owner, controller, key string, value flow.RegisterValue, enforceLimit bool) error {
	if enforceLimit {
		if err := s.checkSize(owner, controller, key, value); err != nil {
			return err
		}
	}

	if err := s.view.Set(owner, controller, key, value); err != nil {
		// wrap error into a fatal error
		setError := errors.NewLedgerFailure(err)
		// wrap with more info
		return fmt.Errorf("failed to update key %s on account %s: %w", PrintableKey(key), hex.EncodeToString([]byte(owner)), setError)
	}

	if enforceLimit {
		if err := s.checkMaxInteraction(); err != nil {
			return err
		}
	}

	if address, isAddress := addressFromOwner(owner); isAddress {
		s.updatedAddresses[address] = struct{}{}
	}

	mapKey := mapKey{owner, controller, key}
	if old, ok := s.updateSize[mapKey]; ok {
		s.WriteCounter--
		s.TotalBytesWritten -= old
	}

	updateSize := uint64(len(owner) + len(controller) + len(key) + len(value))
	s.WriteCounter++
	s.TotalBytesWritten += updateSize
	s.updateSize[mapKey] = updateSize

	return nil
}

// Delete deletes a register
func (s *State) Delete(owner, controller, key string, enforceLimit bool) error {
	return s.Set(owner, controller, key, nil, enforceLimit)
}

// Touch touches a register
func (s *State) Touch(owner, controller, key string) error {
	return s.view.Touch(owner, controller, key)
}

// MeterComputation meters computation usage
func (s *State) MeterComputation(kind, intensity uint) error {
	return s.meter.MeterComputation(kind, intensity)
}

// TotalComputationUsed returns total computation used
func (s *State) TotalComputationUsed() uint {
	return s.meter.TotalComputationUsed()
}

// TotalComputationLimit returns total computation limit
func (s *State) TotalComputationLimit() uint {
	return s.meter.TotalComputationLimit()
}

// MeterMemory meters memory usage
func (s *State) MeterMemory(kind, intensity uint) error {
	return s.meter.MeterMemory(kind, intensity)
}

// TotalMemoryUsed returns total memory used
func (s *State) TotalMemoryUsed() uint {
	return s.meter.TotalMemoryUsed()
}

// TotalMemoryLimit returns total memory limit
func (s *State) TotalMemoryLimit() uint {
	return s.meter.TotalMemoryLimit()
}

// NewChild generates a new child state
func (s *State) NewChild() *State {
	return NewState(s.view.NewChild(),
		WithMeter(s.meter.NewChild()),
		WithMaxKeySizeAllowed(s.maxKeySizeAllowed),
		WithMaxValueSizeAllowed(s.maxValueSizeAllowed),
		WithMaxInteractionSizeAllowed(s.maxInteractionAllowed),
	)
}

// MergeState applies the changes from a the given view to this view.
func (s *State) MergeState(other *State, enforceLimit bool) error {
	err := s.view.MergeView(other.view)
	if err != nil {
		return errors.NewStateMergeFailure(err)
	}

	err = s.meter.MergeMeter(other.meter)
	if err != nil {
		return err
	}

	// apply address updates
	for k, v := range other.updatedAddresses {
		s.updatedAddresses[k] = v
	}

	// apply update sizes
	for k, v := range other.updateSize {
		s.updateSize[k] = v
	}

	// update ledger interactions
	s.ReadCounter += other.ReadCounter
	s.WriteCounter += other.WriteCounter
	s.TotalBytesRead += other.TotalBytesRead
	s.TotalBytesWritten += other.TotalBytesWritten

	// check max interaction as last step
	if enforceLimit {
		return s.checkMaxInteraction()
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

func (s *State) checkMaxInteraction() error {
	if s.InteractionUsed() > s.maxInteractionAllowed {
		return errors.NewLedgerIntractionLimitExceededError(s.InteractionUsed(), s.maxInteractionAllowed)
	}
	return nil
}

func (s *State) checkSize(owner, controller, key string, value flow.RegisterValue) error {
	keySize := uint64(len(owner) + len(controller) + len(key))
	valueSize := uint64(len(value))
	if keySize > s.maxKeySizeAllowed {
		return errors.NewStateKeySizeLimitError(owner, controller, key, keySize, s.maxKeySizeAllowed)
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
func IsFVMStateKey(owner, controller, key string) bool {

	// check if is a service level key (owner and controller is empty)
	// cases:
	// 		- "", "", "uuid"
	// 		- "", "", "account_address_state"
	if len(owner) == 0 && len(controller) == 0 {
		return true
	}
	// check account level keys
	// cases:
	// 		- address, address, "public_key_count"
	// 		- address, address, "public_key_%d" (index)
	// 		- address, address, "contract_names"
	// 		- address, address, "code.%s" (contract name)
	// 		- address, "", exists
	// 		- address, "", "storage_used"
	// 		- address, "", "frozen"
	if owner == controller {
		if key == KeyPublicKeyCount {
			return true
		}
		if bytes.HasPrefix([]byte(key), []byte("public_key_")) {
			return true
		}
		if key == KeyContractNames {
			return true
		}
		if bytes.HasPrefix([]byte(key), []byte(KeyCode)) {
			return true
		}
	}

	if len(controller) == 0 {
		if key == KeyExists {
			return true
		}
		if key == KeyStorageUsed {
			return true
		}
		if key == KeyStorageIndex {
			return true
		}
		if key == KeyAccountFrozen {
			return true
		}
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
