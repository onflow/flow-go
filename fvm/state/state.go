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

type State struct {
	ledger                Ledger
	parent                *State
	touchLog              []payload
	delta                 map[payloadKey]payload
	readCache             map[payloadKey]payload
	updatedAddresses      map[flow.Address]struct{}
	maxKeySizeAllowed     uint64
	maxValueSizeAllowed   uint64
	maxInteractionAllowed uint64
	LedgerInteraction
}

func defaultState(ledger Ledger) *State {
	return &State{
		ledger:                ledger,
		touchLog:              make([]payload, 0),
		delta:                 make(map[payloadKey]payload),
		updatedAddresses:      make(map[flow.Address]struct{}),
		readCache:             make(map[payloadKey]payload),
		maxKeySizeAllowed:     DefaultMaxKeySize,
		maxValueSizeAllowed:   DefaultMaxValueSize,
		maxInteractionAllowed: DefaultMaxInteractionSize,
	}
}

type StateOption func(st *State) *State

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

// TouchLogBytes returns a large byte slice of all register touches
// for read touches the value part is nil for updates
// the value part is also included
func (s *State) TouchLogBytes() []byte {
	res := make([]byte, 0)
	for _, p := range s.touchLog {
		res = append(res, p.bytes()...)
	}
	return res
}

func (s *State) logTouch(pk *payload) {
	s.touchLog = append(s.touchLog, *pk)
}

func (s *State) logTouches(pks []payload) {
	s.touchLog = append(s.touchLog, pks...)
}

// Get returns a register value given owner, controller and key
func (s *State) Get(owner, controller, key string) (flow.RegisterValue, error) {
	if err := s.checkSize(owner, controller, key, []byte{}); err != nil {
		return nil, err
	}

	pKey := payloadKey{owner, controller, key}
	s.logTouch(&payload{pKey, nil})
	// check delta first
	if p, ok := s.delta[pKey]; ok {
		return p.value, nil
	}

	// return from read cache
	if p, ok := s.readCache[pKey]; ok {
		return p.value, nil
	}

	// read from parent
	if s.parent != nil {
		value, err := s.parent.Get(owner, controller, key)
		s.readCache[pKey] = payload{pKey, value}
		return value, err
	}

	// read from ledger
	value, err := s.ledger.Get(owner, controller, key)
	if err != nil {
		return nil, &LedgerFailure{err}
	}

	// update read catch
	p := payload{pKey, value}
	s.readCache[pKey] = p
	s.ReadCounter++
	s.TotalBytesRead += p.size()
	return value, s.checkMaxInteraction()
}

func (s *State) updateDelta(p *payload) {
	// check if a delta already exist for this key
	// reduce the bytes to be written
	if old, ok := s.delta[p.payloadKey]; ok {
		s.ToBeWrittenCounter--
		s.TotalBytesToBeWritten -= old.size()
	}

	s.delta[p.payloadKey] = *p
	s.ToBeWrittenCounter++
	s.TotalBytesToBeWritten += p.size()
}

// Set updates state delta with a register update
func (s *State) Set(owner, controller, key string, value flow.RegisterValue) error {
	if err := s.checkSize(owner, controller, key, value); err != nil {
		return err
	}

	pKey := payloadKey{owner, controller, key}
	p := payload{pKey, value}
	s.logTouch(&p)

	s.updateDelta(&p)

	address, isAddress := addressFromOwner(owner)
	if isAddress {
		s.updatedAddresses[address] = struct{}{}
	}
	return nil
}

func (s *State) Delete(owner, controller, key string) error {
	err := s.Set(owner, controller, key, nil)
	return err
}

// We don't need this later, it should be invisible to the cadence
func (s *State) Touch(owner, controller, key string) error {
	s.logTouch(&payload{payloadKey{owner, controller, key}, nil})
	return nil
}

// NewChild generates a new child state
func (s *State) NewChild() *State {
	return NewState(s.ledger,
		WithParent(s),
		WithMaxKeySizeAllowed(s.maxKeySizeAllowed),
		WithMaxValueSizeAllowed(s.maxValueSizeAllowed),
		WithMaxInteractionSizeAllowed(s.maxInteractionAllowed),
	)
}

func (s *State) MergeTouchLogs(child *State) error {
	// append touches
	s.logTouches(child.touchLog)
	// TODO maybe merge read cache for performance on failed cases
	return nil
}

// MergeState applies the changes from a the given view to this view.
func (s *State) MergeState(child *State) error {
	// append touches
	s.logTouches(child.touchLog)

	// merge read cache
	for k, v := range child.readCache {
		s.readCache[k] = v
	}

	// apply delta
	for _, v := range child.delta {
		s.updateDelta(&v)
	}

	// apply address updates
	for k, v := range child.updatedAddresses {
		s.updatedAddresses[k] = v
	}

	// update ledger interactions
	s.ReadCounter += child.ReadCounter
	s.WriteCounter += child.WriteCounter
	s.TotalBytesRead += child.TotalBytesRead
	s.TotalBytesWritten += child.TotalBytesWritten

	// check max interaction as last step
	return s.checkMaxInteraction()
}

// ApplyDeltaToLedger should only be used for applying changes to ledger at the end of tx
// if successful
func (s *State) ApplyDeltaToLedger() error {
	for _, v := range s.delta {
		s.WriteCounter++
		s.TotalBytesWritten += v.size()
		err := s.ledger.Set(v.owner, v.controller, v.key, v.value)
		if err != nil {
			return err
		}
	}
	return s.checkMaxInteraction()
}

// ApplyTouchesToLedger applies all the register touches to the ledger,
// this is needed for failed transactions
// TODO later we might not need this if we return the touches directly
// to the layer above for SPoCK and data pack construction
func (s *State) ApplyTouchesToLedger() error {
	for _, v := range s.touchLog {
		err := s.ledger.Touch(v.owner, v.controller, v.key)
		if err != nil {
			return err
		}
	}
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

// LedgerInteraction captures stats on how much an state
// interacted with the ledger
type LedgerInteraction struct {
	ReadCounter           uint64
	ToBeWrittenCounter    uint64
	WriteCounter          uint64
	TotalBytesRead        uint64
	TotalBytesToBeWritten uint64
	TotalBytesWritten     uint64
}

func (li *LedgerInteraction) InteractionUsed() uint64 {
	return li.TotalBytesRead + li.TotalBytesWritten
}

type payloadKey struct {
	owner      string
	controller string
	key        string
}

func (pk *payloadKey) size() uint64 {
	return uint64(len(pk.owner) + len(pk.controller) + len(pk.key))
}

type payload struct {
	payloadKey
	value flow.RegisterValue
}

func (p *payload) size() uint64 {
	return uint64(len(p.owner) + len(p.controller) + len(p.key) + len(p.value))
}

func (p *payload) bytes() []byte {
	res := make([]byte, 0)
	res = append(res, []byte(p.owner)...)
	res = append(res, []byte(p.controller)...)
	res = append(res, []byte(p.key)...)
	res = append(res, p.value...)
	return res
}
