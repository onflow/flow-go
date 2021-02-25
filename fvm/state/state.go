package state

import (
	"sort"
	"sync"

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
	ledger Ledger
	// reason for using mutex instead of channel
	// state doesn't have to be thread safe and right now
	// is only used in a single thread but a mutex has been added
	// here to prevent accidental multi-thread use in the future
	lock                  sync.Mutex
	draft                 map[string]payload
	updatedAddresses      map[flow.Address]struct{}
	readCache             map[string]payload
	interactionUsed       uint64
	maxKeySizeAllowed     uint64
	maxValueSizeAllowed   uint64
	maxInteractionAllowed uint64
}

func defaultState(ledger Ledger) *State {
	return &State{
		ledger:                ledger,
		interactionUsed:       uint64(0),
		draft:                 make(map[string]payload),
		updatedAddresses:      make(map[flow.Address]struct{}),
		readCache:             make(map[string]payload),
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

func (s *State) Get(owner, controller, key string) (flow.RegisterValue, error) {
	s.lock.Lock()
	defer s.lock.Unlock()

	if err := s.checkSize(owner, controller, key, []byte{}); err != nil {
		return nil, err
	}

	// check draft first
	if p, ok := s.draft[fullKey(owner, controller, key)]; ok {
		// just call the ledger get for tracking touches
		_, err := s.ledger.Get(owner, controller, key)
		if err != nil {
			return nil, &LedgerFailure{err}
		}
		return p.value, nil
	}

	// return from draft
	if p, ok := s.draft[fullKey(owner, controller, key)]; ok {
		// just call the ledger get for tracking touches
		_, err := s.ledger.Get(owner, controller, key)
		if err != nil {
			return nil, &LedgerFailure{err}
		}
		return p.value, nil
	}

	// return from read cache
	if p, ok := s.readCache[fullKey(owner, controller, key)]; ok {
		// just call the ledger get for tracking touches
		_, err := s.ledger.Get(owner, controller, key)
		if err != nil {
			return nil, &LedgerFailure{err}
		}
		return p.value, nil
	}

	// read from ledger
	value, err := s.ledger.Get(owner, controller, key)
	if err != nil {
		return nil, &LedgerFailure{err}
	}

	// update read catch
	s.readCache[fullKey(owner, controller, key)] = payload{owner, controller, key, value}

	return value, s.updateInteraction(owner, controller, key, value, []byte{})
}

func (s *State) Set(owner, controller, key string, value flow.RegisterValue) error {
	s.lock.Lock()
	defer s.lock.Unlock()

	if err := s.checkSize(owner, controller, key, value); err != nil {
		return err
	}

	s.draft[fullKey(owner, controller, key)] = payload{owner, controller, key, value}
	address, isAddress := addressFromOwner(owner)
	if isAddress {
		s.updatedAddresses[address] = struct{}{}
	}
	return nil
}

func (s *State) Touch(owner, controller, key string) error {
	_, err := s.Get(owner, controller, key)
	return err
}

func (s *State) Delete(owner, controller, key string) error {
	err := s.Set(owner, controller, key, nil)
	return err
}

func (s *State) Commit() error {
	s.lock.Lock()
	defer s.lock.Unlock()

	// TODO right now we sort keys to minimize
	// the impact on the spock, but later we might
	// expose interactionFingerprint as a separate
	// parameters, this preserve number of reads and
	// order of reads as extra layer of entropy

	keys := make([]string, 0)
	for k := range s.draft {
		keys = append(keys, k)
	}

	sort.Strings(keys)

	for _, k := range keys {
		p := s.draft[k]
		oldValue, err := s.ledger.Get(p.owner, p.controller, p.key)
		if err != nil {
			return err
		}
		// update interaction
		err = s.updateInteraction(p.owner, p.controller, p.key, oldValue, p.value)
		if err != nil {
			return err
		}
		err = s.ledger.Set(p.owner, p.controller, p.key, p.value)
		if err != nil {
			return err
		}

		// update read cache
		s.readCache[fullKey(p.owner, p.controller, p.key)] = p
	}

	// reset draft
	s.draft = make(map[string]payload)
	s.updatedAddresses = make(map[flow.Address]struct{})

	return nil
}

func (s *State) Rollback() error {
	s.lock.Lock()
	defer s.lock.Unlock()

	s.draft = make(map[string]payload)
	s.updatedAddresses = make(map[flow.Address]struct{})
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
	s.lock.Lock()
	defer s.lock.Unlock()
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

type payload struct {
	owner      string
	controller string
	key        string
	value      flow.RegisterValue
}
