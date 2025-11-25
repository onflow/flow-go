package heropool

import (
	"fmt"
	"math"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/utils/rand"
)

type EjectionMode string

const (
	RandomEjection = EjectionMode("random-ejection")
	LRUEjection    = EjectionMode("lru-ejection")
	NoEjection     = EjectionMode("no-ejection")
)

// StateIndex is a type of state of a placeholder in a pool.
type StateIndex uint

const numberOfStates = 2
const ( // iota is reset to 0
	stateFree StateIndex = iota
	stateUsed
)

// EIndex is data type representing an value index in Pool.
type EIndex uint32

// InvalidIndex is used when a link doesn't point anywhere, in other words it is an equivalent of a nil address.
const InvalidIndex EIndex = math.MaxUint32

// poolEntity represents the data type that is maintained by
type poolEntity[K comparable, V any] struct {
	PoolEntity[K, V]
	// owner maintains an external reference to the key associated with this value.
	// The key is maintained by the HeroCache, and value is maintained by Pool.
	owner uint64

	// node keeps the link to the previous and next entities.
	// When this value is allocated, the node maintains the connections it to the next and previous (used) pool entities.
	// When this value is unallocated, the node maintains the connections to the next and previous unallocated (free) pool entities.
	node link

	// invalidated indicates whether this pool value has been invalidated.
	// A value becomes invalidated when it is removed or ejected from the pool,
	// meaning its key and value are no longer valid for use.
	// This flag helps manage the lifecycle of the value within the pool.
	invalidated bool
}

type PoolEntity[K comparable, V any] struct {
	// Key associated with this value.
	key K

	// Actual value itself.
	value V
}

func (p PoolEntity[K, V]) Id() K {
	return p.key
}

func (p PoolEntity[K, V]) Entity() V {
	return p.value
}

type Pool[K comparable, V any] struct {
	logger       zerolog.Logger
	states       []state // keeps track of a slot's state
	poolEntities []poolEntity[K, V]
	ejectionMode EjectionMode
}

// NewHeroPool returns a pointer to a new hero pool constructed based on a provided EjectionMode,
// logger and a provided fixed size.
func NewHeroPool[K comparable, V any](sizeLimit uint32, ejectionMode EjectionMode, logger zerolog.Logger) *Pool[K, V] {
	l := &Pool[K, V]{
		//construcs states initialized to InvalidIndexes
		states:       NewStates(numberOfStates),
		poolEntities: make([]poolEntity[K, V], sizeLimit),
		ejectionMode: ejectionMode,
		logger:       logger,
	}

	l.setDefaultNodeLinkValues()
	l.initFreeEntities()

	return l
}

// setDefaultNodeLinkValues sets nodes prev and next to InvalidIndex for all cached entities in poolEntities.
func (p *Pool[K, V]) setDefaultNodeLinkValues() {
	for i := 0; i < len(p.poolEntities); i++ {
		p.poolEntities[i].node.next = InvalidIndex
		p.poolEntities[i].node.prev = InvalidIndex
	}
}

// initFreeEntities initializes the free double linked-list with the indices of all cached entity poolEntities.
func (p *Pool[K, V]) initFreeEntities() {
	p.states[stateFree].head = 0
	p.states[stateFree].tail = 0
	p.poolEntities[p.states[stateFree].head].node.prev = InvalidIndex
	p.poolEntities[p.states[stateFree].tail].node.next = InvalidIndex
	p.states[stateFree].size = 1
	for i := 1; i < len(p.poolEntities); i++ {
		p.connect(p.states[stateFree].tail, EIndex(i))
		p.states[stateFree].tail = EIndex(i)
		p.poolEntities[p.states[stateFree].tail].node.next = InvalidIndex
		p.states[stateFree].size++
	}
}

// Add writes given value into a poolEntity on the underlying values linked-list.
//
// The boolean return value (slotAvailable) says whether pool has an available slot. Pool goes out of available slots if
// it is full and no ejection is set.
//
// If the pool has no available slots and an ejection is set, ejection occurs when adding a new value.
// If an ejection occurred, ejectedEntity holds the ejected value.
//
// Returns:
//   - valueIndex: The index in the pool where the new entity was inserted.
//     If no slot is available (and no ejection occurs), this will be set to InvalidIndex.
//   - slotAvailable: Indicates whether an available slot was found. It is true if
//     the entity was inserted (either in a free slot or by ejecting an existing entity).
//   - ejectedValue: If an ejection occurred to free a slot, this value holds the entity
//     that was ejected; otherwise, it is the zero value of type V.
//   - wasEjected: Indicates whether an ejection was performed (true if an entity was ejected,
//     false if the insertion simply used an available free slot).
func (p *Pool[K, V]) Add(key K, value V, owner uint64) (
	valueIndex EIndex, slotAvailable bool, ejectedValue V, wasEjected bool) {
	valueIndex, slotAvailable, ejectedValue, wasEjected = p.sliceIndexForEntity()
	if slotAvailable {
		p.poolEntities[valueIndex].value = value
		p.poolEntities[valueIndex].key = key
		p.poolEntities[valueIndex].owner = owner
		// Reset the invalidated flag when reusing a slot.
		p.poolEntities[valueIndex].invalidated = false
		p.switchState(stateFree, stateUsed, valueIndex)
	}

	return valueIndex, slotAvailable, ejectedValue, wasEjected
}

// Get returns value corresponding to the value index from the underlying list.
func (p *Pool[K, V]) Get(valueIndex EIndex) (K, V, uint64) {
	return p.poolEntities[valueIndex].key, p.poolEntities[valueIndex].value, p.poolEntities[valueIndex].owner
}

// All returns all stored values in this pool.
func (p *Pool[K, V]) All() []PoolEntity[K, V] {
	all := make([]PoolEntity[K, V], p.states[stateUsed].size)
	next := p.states[stateUsed].head

	for i := uint32(0); i < p.states[stateUsed].size; i++ {
		e := p.poolEntities[next]
		all[i] = e.PoolEntity
		next = e.node.next
	}

	return all
}

// Head returns the head of used items. Assuming no ejection happened and pool never goes beyond limit, Head returns
// the first inserted key and value of the element.
func (p *Pool[K, V]) Head() (key K, value V, ok bool) {
	if p.states[stateUsed].size == 0 {
		return key, value, false
	}
	e := p.poolEntities[p.states[stateUsed].head]
	return e.Id(), e.Entity(), true
}

// sliceIndexForEntity returns a slice index which hosts the next entity to be added to the list.
// This index is invalid if there are no available slots or ejection could not be performed.
// If the valid index is returned then it is guaranteed that it corresponds to a free list head.
// Thus, when filled with a new value a switchState must be applied.
//
// The first boolean return value (hasAvailableSlot) says whether pool has an available slot.
// Pool goes out of available slots if it is full and no ejection is set.
//
// Ejection happens if there is no available slot, and there is an ejection mode set.
// If an ejection occurred, ejectedEntity holds the ejected entity.
//
// Returns:
//   - i: The slice index where the new entity should be stored. This index is valid only
//     if hasAvailableSlot is true.
//   - hasAvailableSlot: Indicates whether the pool has an available slot for a new entity.
//     If false, the pool is full and no ejection was performed (e.g. if ejection mode is NoEjection).
//   - ejectedValue: If an ejection occurred to free up a slot, this value holds the entity that was
//     removed (ejected) from the pool. Otherwise, it is the zero value of type V.
//   - wasEjected (bool): Indicates whether an ejection occurred (true if an entity was ejected; false otherwise).
func (p *Pool[K, V]) sliceIndexForEntity() (i EIndex, hasAvailableSlot bool, ejectedValue V, wasEjected bool) {
	lruEject := func() (EIndex, bool, V, bool) {
		// LRU ejection
		// the used head is the oldest entity, so we turn the used head to a free head here.
		invalidatedValue := p.invalidateUsedHead()
		return p.states[stateFree].head, true, invalidatedValue, true
	}

	if p.states[stateFree].size == 0 {
		// the free list is empty, so we are out of space, and we need to eject.
		switch p.ejectionMode {
		case NoEjection:
			// pool is set for no ejection, hence, no slice index is selected, abort immediately.
			return InvalidIndex, false, ejectedValue, false
		case RandomEjection:
			// we only eject randomly when the pool is full and random ejection is on.
			random, err := rand.Uint32n(p.states[stateUsed].size)
			if err != nil {
				p.logger.Fatal().Err(err).
					Msg("hero pool random ejection failed - falling back to LRU ejection")
				// fall back to LRU ejection only for this instance
				return lruEject()
			}
			randomIndex := EIndex(random)
			invalidatedEntity := p.invalidateValueAtIndex(randomIndex)
			return p.states[stateFree].head, true, invalidatedEntity, true
		case LRUEjection:
			// LRU ejection
			return lruEject()
		}
	}

	// returning the head of free list as the slice index for the next entity to be added
	return p.states[stateFree].head, true, ejectedValue, false
}

// Size returns total number of values that this list maintains.
func (p *Pool[K, V]) Size() uint32 {
	return p.states[stateUsed].size
}

// getHeads returns values corresponding to the used and free heads.
func (p *Pool[K, V]) getHeads() (*poolEntity[K, V], *poolEntity[K, V]) {
	var usedHead, freeHead *poolEntity[K, V]
	if p.states[stateUsed].size != 0 {
		usedHead = &p.poolEntities[p.states[stateUsed].head]
	}

	if p.states[stateFree].size != 0 {
		freeHead = &p.poolEntities[p.states[stateFree].head]
	}

	return usedHead, freeHead
}

// getTails returns values corresponding to the used and free tails.
func (p *Pool[K, V]) getTails() (*poolEntity[K, V], *poolEntity[K, V]) {
	var usedTail, freeTail *poolEntity[K, V]
	if p.states[stateUsed].size != 0 {
		usedTail = &p.poolEntities[p.states[stateUsed].tail]
	}
	if p.states[stateFree].size != 0 {
		freeTail = &p.poolEntities[p.states[stateFree].tail]
	}

	return usedTail, freeTail
}

// connect links the prev and next nodes as the adjacent nodes in the double-linked list.
func (p *Pool[K, V]) connect(prev EIndex, next EIndex) {
	p.poolEntities[prev].node.next = next
	p.poolEntities[next].node.prev = prev
}

// invalidateUsedHead moves current used head forward by one node. It
// also removes the value the invalidated head is presenting and appends the
// node represented by the used head to the tail of the free list.
func (p *Pool[K, V]) invalidateUsedHead() V {
	headSliceIndex := p.states[stateUsed].head
	return p.invalidateValueAtIndex(headSliceIndex)
}

// Remove removes value corresponding to given getSliceIndex from the list.
func (p *Pool[K, V]) Remove(sliceIndex EIndex) V {
	return p.invalidateValueAtIndex(sliceIndex)
}

// UpdateAtIndex replaces the value at the given pool index.
func (p *Pool[K, V]) UpdateAtIndex(idx EIndex, newValue V) {
	p.poolEntities[idx].value = newValue
}

// Touch marks the entry at pool index as “recently used” by moving it
// from the head of the used list to its tail.
func (p *Pool[K, V]) Touch(idx EIndex) {
	// remove from used list
	p.switchState(stateUsed, stateFree, idx)
	// immediately append back to used list tail
	p.switchState(stateFree, stateUsed, idx)
}

// invalidateValueAtIndex invalidates the given getSliceIndex in the linked list by
// removing its corresponding linked-list node from the used linked list, and appending
// it to the tail of the free list. It also removes the value that the invalidated node is presenting.
func (p *Pool[K, V]) invalidateValueAtIndex(sliceIndex EIndex) V {
	invalidatedValue := p.poolEntities[sliceIndex].value
	if p.poolEntities[sliceIndex].invalidated {
		panic(fmt.Sprintf("removing an entity from an empty slot with an index : %d", sliceIndex))
	}
	p.switchState(stateUsed, stateFree, sliceIndex)

	var zeroKey K
	var zeroValue V
	p.poolEntities[sliceIndex].key = zeroKey
	p.poolEntities[sliceIndex].value = zeroValue
	p.poolEntities[sliceIndex].invalidated = true
	return invalidatedValue
}

// isInvalidated returns true if linked-list node represented by getSliceIndex does not contain
// a valid value.
func (p *Pool[K, V]) isInvalidated(sliceIndex EIndex) bool {
	return p.poolEntities[sliceIndex].invalidated
}

// switches state of a value.
func (p *Pool[K, V]) switchState(stateFrom StateIndex, stateTo StateIndex, valueIndex EIndex) {
	// Remove from stateFrom list
	if p.states[stateFrom].size == 0 {
		panic("Removing an entity from an empty list")
	} else if p.states[stateFrom].size == 1 {
		p.states[stateFrom].head = InvalidIndex
		p.states[stateFrom].tail = InvalidIndex
	} else {
		node := p.poolEntities[valueIndex].node

		if valueIndex != p.states[stateFrom].head && valueIndex != p.states[stateFrom].tail {
			// links next and prev elements for non-head and non-tail element
			p.connect(node.prev, node.next)
		}

		if valueIndex == p.states[stateFrom].head {
			// moves head forward
			p.states[stateFrom].head = node.next
			p.poolEntities[p.states[stateFrom].head].node.prev = InvalidIndex
		}

		if valueIndex == p.states[stateFrom].tail {
			// moves tail backwards
			p.states[stateFrom].tail = node.prev
			p.poolEntities[p.states[stateFrom].tail].node.next = InvalidIndex
		}
	}
	p.states[stateFrom].size--

	// Add to stateTo list
	if p.states[stateTo].size == 0 {
		p.states[stateTo].head = valueIndex
		p.states[stateTo].tail = valueIndex
		p.poolEntities[p.states[stateTo].head].node.prev = InvalidIndex
		p.poolEntities[p.states[stateTo].tail].node.next = InvalidIndex
	} else {
		p.connect(p.states[stateTo].tail, valueIndex)
		p.states[stateTo].tail = valueIndex
		p.poolEntities[p.states[stateTo].tail].node.next = InvalidIndex
	}
	p.states[stateTo].size++
}
