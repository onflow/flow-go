package state

import (
	"github.com/onflow/flow-go/model/flow"
)

// TODO we started with high numbers here and we might
// tune (reduce) them when we have more data
const (
	DefaultMaxKeySize         = 16_000        // ~16KB
	DefaultMaxValueSize       = 256_000_000   // ~256MB
	DefaultMaxInteractionSize = 2_000_000_000 // ~2GB
)

type mapKey struct {
	owner, controller, key string
}

// State represents the execution state
// it holds draft of updates and captures
// all register touches
type State struct {
	view                  View
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
		updatedAddresses:      make(map[flow.Address]struct{}),
		updateSize:            make(map[mapKey]uint64),
		maxKeySizeAllowed:     DefaultMaxKeySize,
		maxValueSizeAllowed:   DefaultMaxValueSize,
		maxInteractionAllowed: DefaultMaxInteractionSize,
	}
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

func (s *State) InteractionUsed() uint64 {
	return s.TotalBytesRead + s.TotalBytesWritten
}

// Get returns a register value given owner, controller and key
func (s *State) Get(owner, controller, key string) (flow.RegisterValue, error) {
	if err := s.checkSize(owner, controller, key, []byte{}); err != nil {
		return nil, err
	}

	value, err := s.view.Get(owner, controller, key)
	if err != nil {
		return nil, err
	}

	s.ReadCounter++
	s.TotalBytesRead += uint64(len(value))

	return value, s.checkMaxInteraction()
}

// Set updates state delta with a register update
func (s *State) Set(owner, controller, key string, value flow.RegisterValue) error {
	if err := s.checkSize(owner, controller, key, value); err != nil {
		return err
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

	if err := s.checkMaxInteraction(); err != nil {
		return err
	}

	address, isAddress := addressFromOwner(owner)
	if isAddress {
		s.updatedAddresses[address] = struct{}{}
	}
	return nil
}

func (s *State) Delete(owner, controller, key string) error {
	return s.Set(owner, controller, key, nil)
}

// We don't need this later, it should be invisible to the cadence
func (s *State) Touch(owner, controller, key string) error {
	return s.view.Touch(owner, controller, key)
}

// NewChild generates a new child state
func (s *State) NewChild() *State {
	return NewState(s.view,
		WithMaxKeySizeAllowed(s.maxKeySizeAllowed),
		WithMaxValueSizeAllowed(s.maxValueSizeAllowed),
		WithMaxInteractionSizeAllowed(s.maxInteractionAllowed),
	)
}

// MergeState applies the changes from a the given view to this view.
func (s *State) MergeState(other *State) error {

	s.view.MergeView(other.view)

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
	return s.checkMaxInteraction()
}

func (s *State) UpdatedAddresses() []flow.Address {
	addresses := make([]flow.Address, 0, len(s.updatedAddresses))
	for k := range s.updatedAddresses {
		addresses = append(addresses, k)
	}
	return addresses
}

func (s *State) checkMaxInteraction() error {
	if s.InteractionUsed() > s.maxInteractionAllowed {
		return &StateInteractionLimitExceededError{
			Used:  s.InteractionUsed(),
			Limit: s.maxInteractionAllowed}
	}
	return nil
}

func (s *State) checkSize(owner, controller, key string, value flow.RegisterValue) error {
	keySize := uint64(len(owner) + len(controller) + len(key))
	valueSize := uint64(len(value))
	if keySize > s.maxKeySizeAllowed {
		return &StateKeySizeLimitError{Owner: owner,
			Controller: controller,
			Key:        key,
			Size:       keySize,
			Limit:      s.maxKeySizeAllowed}
	}
	if valueSize > s.maxValueSizeAllowed {
		return &StateValueSizeLimitError{Value: value,
			Size:  keySize,
			Limit: s.maxKeySizeAllowed}
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
