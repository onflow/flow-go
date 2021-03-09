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

type StateOption func(st *State) *State

type State struct {
	ledger                Ledger
	parent                *State
	touchLog              []payload
	changeLog             []payload
	delta                 map[payloadKey]payload
	readCache             map[payloadKey]payload
	updatedAddresses      map[flow.Address]struct{}
	interactionUsed       uint64
	maxKeySizeAllowed     uint64
	maxValueSizeAllowed   uint64
	maxInteractionAllowed uint64
}

func defaultState(ledger Ledger) *State {
	return &State{
		ledger:                ledger,
		interactionUsed:       uint64(0),
		touchLog:              make([]payload, 0),
		changeLog:             make([]payload, 0),
		delta:                 make(map[payloadKey]payload),
		updatedAddresses:      make(map[flow.Address]struct{}),
		readCache:             make(map[payloadKey]payload),
		maxKeySizeAllowed:     DefaultMaxKeySize,
		maxValueSizeAllowed:   DefaultMaxValueSize,
		maxInteractionAllowed: DefaultMaxInteractionSize,
	}
}

// NewState constructs a new state
func NewState(ledger Ledger, opts ...StateOption) *State {
	ctx := defaultState(ledger)
	for _, applyOption := range opts {
		ctx = applyOption(ctx)
	}
	return ctx
}

// WithParent sets a parent for the state
func WithParent(parent *State) func(st *State) *State {
	return func(st *State) *State {
		st.parent = parent
		return st
	}
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

func (s *State) logTouch(owner, controller, key string, value flow.RegisterValue) {
	s.touchLog = append(s.touchLog, payload{payloadKey{owner, controller, key}, value})
}

func (s *State) logChange(owner, controller, key string, value flow.RegisterValue) {
	s.changeLog = append(s.changeLog, payload{payloadKey{owner, controller, key}, value})
}

// Get returns a register value given owner, controller and key
func (s *State) Get(owner, controller, key string) (flow.RegisterValue, error) {
	if err := s.checkSize(owner, controller, key, []byte{}); err != nil {
		return nil, err
	}

	pKey := payloadKey{owner, controller, key}

	// check delta first
	if p, ok := s.delta[pKey]; ok {
		s.logTouch(owner, controller, key, p.value)
		return p.value, nil
	}

	// return from read cache
	if p, ok := s.readCache[pKey]; ok {
		s.logTouch(owner, controller, key, p.value)
		return p.value, nil
	}

	// read from parent
	if s.parent != nil {
		value, err := s.parent.Get(owner, controller, key)
		s.readCache[pKey] = payload{pKey, value}
		s.logTouch(owner, controller, key, value)
		return value, err
	}

	// read from ledger
	value, err := s.ledger.Get(owner, controller, key)
	if err != nil {
		return nil, &LedgerFailure{err}
	}

	// update read catch
	s.readCache[pKey] = payload{pKey, value}
	s.logTouch(owner, controller, key, value)
	return value, s.updateInteraction(owner, controller, key, value, []byte{})
}

// Set updates state delta with a register update
func (s *State) Set(owner, controller, key string, value flow.RegisterValue) error {
	if err := s.checkSize(owner, controller, key, value); err != nil {
		return err
	}

	pKey := payloadKey{owner, controller, key}

	s.delta[pKey] = payload{pKey, value}
	s.logTouch(owner, controller, key, value)
	s.logChange(owner, controller, key, value)
	address, isAddress := addressFromOwner(owner)
	if isAddress {
		s.updatedAddresses[address] = struct{}{}
	}
	return nil
}

func (s *State) Touch(owner, controller, key string) error {
	// TODO we don't need to call the ledger touch, touch can be returned later form the logs
	err := s.Touch(owner, controller, key)
	// TODO figure out value instead of nil
	s.logTouch(owner, controller, key, nil)
	return err
}

func (s *State) Delete(owner, controller, key string) error {
	err := s.Set(owner, controller, key, nil)
	s.logTouch(owner, controller, key, nil)
	s.logChange(owner, controller, key, nil)
	return err
}

// NewChild generates a new child state
func (s *State) NewChild() *State {
	return NewState(s.ledger, WithParent(s))
}

// MergeState applies the changes from a the given view to this view.
// TODO rename this, this is not actually a merge as we can't merge
// ledger
func (s *State) MergeState(child *State) {
	// transfer touches
	for _, l := range child.touchLog {
		// TODO do this through touch method
		s.logTouch(l.owner, l.controller, l.key, l.value)
	}

	// transfer changes
	for _, l := range child.changeLog {
		s.Set(l.owner, l.controller, l.key, l.value)
	}
}

// ApplyDeltaToLedger should only be used for applying changes to ledger at the end of tx
// if successful
func (s *State) ApplyDeltaToLedger() error {

	// TODO
	// we can change this to only use delta
	// when SPoCK secret is moved to lower level
	for _, v := range s.changeLog {
		err := s.ledger.Set(v.owner, v.controller, v.key, v.value)
		if err != nil {
			return err
		}
	}

	// TODO clean up change log and others if needed to be called multiple times in the future
	return nil
}

func (s *State) Ledger() Ledger {
	return s.ledger
}

func (s *State) UpdatedAddresses() []flow.Address {
	addresses := make([]flow.Address, 0, len(s.updatedAddresses))
	for k := range s.updatedAddresses {
		addresses = append(addresses, k)
	}
	return addresses
}

func (s *State) InteractionUsed() uint64 {
	return s.interactionUsed
}

func (s *State) updateInteraction(owner, controller, key string, oldValue, newValue flow.RegisterValue) error {
	keySize := uint64(len(owner) + len(controller) + len(key))
	oldValueSize := uint64(len(oldValue))
	newValueSize := uint64(len(newValue))
	s.interactionUsed += keySize + oldValueSize + newValueSize
	if s.interactionUsed > s.maxInteractionAllowed {
		return &StateInteractionLimitExceededError{
			Used:  s.interactionUsed,
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

type payloadKey struct {
	owner      string
	controller string
	key        string
}
type payload struct {
	payloadKey
	value flow.RegisterValue
}
