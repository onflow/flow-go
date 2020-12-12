package state

import (
	"github.com/onflow/flow-go/model/flow"
)

const (
	defaultMaxKeySize         = 16_000     // ~16KB
	defaultMaxValueSize       = 32_000_000 // ~32MB
	defaultMaxInteractionSize = 64_000_000 // ~64MB
)

type StateOption func(st *State) *State
type State struct {
	ledger                Ledger
	draft                 map[string]payload
	interactionUsed       uint64
	maxKeySizeAllowed     uint64
	maxValueSizeAllowed   uint64
	maxInteractionAllowed uint64
}

func defaultState(ledger Ledger) *State {
	return &State{
		ledger:                ledger,
		interactionUsed:       uint64(0),
		draft:                 make(map[string]payload, 0),
		maxKeySizeAllowed:     defaultMaxKeySize,
		maxValueSizeAllowed:   defaultMaxValueSize,
		maxInteractionAllowed: defaultMaxInteractionSize,
	}
}

// NewState constucts a new state
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

// WithMaxInteractionAllowed sets limit on total byte interaction with ledger
func WithMaxInteractionAllowed(limit uint64) func(st *State) *State {
	return func(st *State) *State {
		st.maxInteractionAllowed = limit
		return st
	}
}

func (s State) Read(owner, controller, key string) (flow.RegisterValue, error) {
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

	value, err := s.ledger.Get(owner, controller, key)
	if err != nil {
		return nil, &LedgerFailure{err}
	}
	// update interaction
	keySize := uint64(len(owner) + len(controller) + len(key))
	valueSize := uint64(len(value))
	err = s.updateInteraction(keySize, valueSize, 0)
	if err != nil {
		return nil, &LedgerFailure{err}
	}

	return value, nil
}

func (s State) Update(owner, controller, key string, value flow.RegisterValue) error {
	if err := s.checkSize(owner, controller, key, value); err != nil {
		return err
	}
	s.draft[fullKey(owner, controller, key)] = payload{owner, controller, key, value}
	// TODO add interaction update
	return nil
}

func (s *State) Commit() error {
	// send all of the key values to the set
	for _, p := range s.draft {
		err := s.ledger.Set(p.owner, p.controller, p.key, p.value)
		if err != nil {
			return &LedgerFailure{err}
		}
	}
	return nil
}

func (s *State) Rollback() error {
	s.draft = make(map[string]payload, 0)
	return nil
}

func (s *State) Ledger() Ledger {
	return s.ledger
}

func (s State) updateInteraction(keySize, oldValueSize, newValueSize uint64) error {
	s.interactionUsed += keySize + oldValueSize + newValueSize
	if s.interactionUsed > s.maxInteractionAllowed {
		return &StateInteractionLimitExceededError{
			Used:  s.interactionUsed,
			Limit: s.maxInteractionAllowed}
	}
	return nil
}

func (s State) checkSize(owner, controller, key string, value flow.RegisterValue) error {
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

func (s State) InteractionUsed() uint64 {
	return s.interactionUsed
}

type payload struct {
	owner      string
	controller string
	key        string
	value      flow.RegisterValue
}
