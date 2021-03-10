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

// State represents the execution state
// it holds draft of updates and captures
// all register touches
type State struct {
	ledger   Ledger
	parent   *State
	touchLog []Payload
	// touchHasher           hash.Hasher
	delta                 map[PayloadKey]Payload
	readCache             map[PayloadKey]Payload
	updatedAddresses      map[flow.Address]struct{}
	maxKeySizeAllowed     uint64
	maxValueSizeAllowed   uint64
	maxInteractionAllowed uint64
	LedgerInteraction
}

func defaultState(ledger Ledger) *State {
	return &State{
		ledger:   ledger,
		touchLog: make([]Payload, 0),
		// touchHasher:           hash.NewSHA3_256(),
		delta:                 make(map[PayloadKey]Payload),
		updatedAddresses:      make(map[flow.Address]struct{}),
		readCache:             make(map[PayloadKey]Payload),
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

// // TouchHash returns the hash of all touches
// // for read touches the value part is nil for updates
// // the value part is also included
// func (s *State) TouchHash() []byte {
// 	return s.touchHasher.SumHash()
// }

func (s *State) Touches() []Payload {
	return s.touchLog
}

func (s *State) LogTouch(pk *Payload) {
	// _, err := s.touchHasher.Write(pk.bytes())
	// if err != nil {
	// 	// TODO return error
	// 	panic(err)
	// 	// return fmt.Errorf("error updating spock secret data: %w", err)
	// }

	s.touchLog = append(s.touchLog, *pk)
}

func (s *State) LogTouches(pks []Payload) {
	// TODO make this smarter through append
	for _, t := range pks {
		s.LogTouch(&t)
	}
}

// Get returns a register value given owner, controller and key
func (s *State) Get(owner, controller, key string) (flow.RegisterValue, error) {
	if err := s.checkSize(owner, controller, key, []byte{}); err != nil {
		return nil, err
	}

	pKey := PayloadKey{owner, controller, key}
	s.LogTouch(&Payload{pKey, nil})

	value, err := s.get(pKey)
	if err != nil {
		return nil, err
	}
	return value, s.checkMaxInteraction()
}

func (s *State) getWithoutTracking(owner, controller, key string) (flow.RegisterValue, error) {
	pKey := PayloadKey{owner, controller, key}
	value, err := s.get(pKey)
	if err != nil {
		return nil, err
	}
	return value, nil
}

// Get returns a register value given owner, controller and key
func (s *State) get(pKey PayloadKey) (flow.RegisterValue, error) {
	// check delta first
	if p, ok := s.delta[pKey]; ok {
		return p.Value, nil
	}

	// return from read cache
	if p, ok := s.readCache[pKey]; ok {
		return p.Value, nil
	}

	// read from parent
	if s.parent != nil {
		value, err := s.parent.getWithoutTracking(pKey.Owner, pKey.Controller, pKey.Key)
		p := Payload{pKey, value}
		s.readCache[pKey] = p
		return value, err
	}

	// read from ledger
	value, err := s.ledger.Get(pKey.Owner, pKey.Controller, pKey.Key)
	if err != nil {
		return nil, &LedgerFailure{err}
	}
	p := Payload{pKey, value}
	s.readCache[pKey] = p
	s.ReadCounter++
	s.TotalBytesRead += p.size()
	return value, nil
}

func (s *State) updateDelta(p *Payload) {
	// check if a delta already exist for this key
	// reduce the bytes to be written
	if old, ok := s.delta[p.PayloadKey]; ok {
		s.ToBeWrittenCounter--
		s.TotalBytesToBeWritten -= old.size()
	}

	s.delta[p.PayloadKey] = *p
	s.ToBeWrittenCounter++
	s.TotalBytesToBeWritten += p.size()
}

// Set updates state delta with a register update
func (s *State) Set(owner, controller, key string, value flow.RegisterValue) error {
	if err := s.checkSize(owner, controller, key, value); err != nil {
		return err
	}

	pKey := PayloadKey{owner, controller, key}
	p := Payload{pKey, value}
	s.LogTouch(&p)

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
	s.LogTouch(&Payload{PayloadKey{owner, controller, key}, nil})
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
	s.LogTouches(child.touchLog)
	// TODO maybe merge read cache for performance on failed cases
	return nil
}

// MergeAnyState applies the changes from any given state
func (s *State) MergeAnyState(other *State) error {
	// append touches
	s.LogTouches(other.touchLog)

	// apply delta
	for _, v := range other.delta {
		s.updateDelta(&v)
	}

	// apply address updates
	for k, v := range other.updatedAddresses {
		s.updatedAddresses[k] = v
	}

	// update ledger interactions
	s.ReadCounter += other.ReadCounter
	s.WriteCounter += other.WriteCounter
	s.TotalBytesRead += other.TotalBytesRead
	s.TotalBytesWritten += other.TotalBytesWritten

	// check max interaction as last step
	return s.checkMaxInteraction()
}

// MergeState applies the changes from a the given view to this view.
func (s *State) MergeState(child *State) error {
	// merge read cache
	for k, v := range child.readCache {
		s.readCache[k] = v
	}

	return s.MergeAnyState(child)
}

// ApplyDeltaToLedger should only be used for applying changes to ledger at the end of tx
// if successful
func (s *State) ApplyDeltaToLedger() error {
	for _, v := range s.delta {
		s.WriteCounter++
		s.TotalBytesWritten += v.size()
		err := s.ledger.Set(v.Owner, v.Controller, v.Key, v.Value)
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
		err := s.ledger.Touch(v.Owner, v.Controller, v.Key)
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

type PayloadKey struct {
	Owner      string
	Controller string
	Key        string
}

type Payload struct {
	PayloadKey
	Value flow.RegisterValue
}

func (p *Payload) size() uint64 {
	return uint64(len(p.Owner) + len(p.Controller) + len(p.Key) + len(p.Value))
}

func (p *Payload) bytes() []byte {
	res := make([]byte, 0)
	res = append(res, []byte(p.Owner)...)
	res = append(res, []byte(p.Controller)...)
	res = append(res, []byte(p.Key)...)
	res = append(res, p.Value...)
	return res
}
