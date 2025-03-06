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

// StateIndex is a type of a state of a placeholder in a pool.
type StateIndex uint

const numberOfStates = 2
const ( // iota is reset to 0
	stateFree StateIndex = iota
	stateUsed
)

// EIndex is data type representing an entity index in Pool.
type EIndex uint32

// InvalidIndex is used when a link doesnt point anywhere, in other words it is an equivalent of a nil address.
const InvalidIndex EIndex = math.MaxUint32

// poolEntity represents the data type that is maintained by
type poolEntity[K comparable, V any] struct {
	PoolEntity[K, V]
	// owner maintains an external reference to the key associated with this entity.
	// The key is maintained by the HeroCache, and entity is maintained by Pool.
	owner uint64

	// node keeps the link to the previous and next entities.
	// When this entity is allocated, the node maintains the connections it to the next and previous (used) pool entities.
	// When this entity is unallocated, the node maintains the connections to the next and previous unallocated (free) pool entities.
	node link
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
func (p *Pool[K, V]) Add(key K, value V, owner uint64) (
	entityIndex EIndex, slotAvailable bool, ejectedEntity V) {
	entityIndex, slotAvailable, ejectedEntity = p.sliceIndexForEntity()
	if slotAvailable {
		p.poolEntities[entityIndex].value = value
		p.poolEntities[entityIndex].key = key
		p.poolEntities[entityIndex].owner = owner
		p.switchState(stateFree, stateUsed, entityIndex)
	}

	return entityIndex, slotAvailable, ejectedEntity
}

// Get returns entity corresponding to the entity index from the underlying list.
func (p *Pool[K, V]) Get(entityIndex EIndex) (K, V, uint64) {
	return p.poolEntities[entityIndex].key, p.poolEntities[entityIndex].value, p.poolEntities[entityIndex].owner
}

// All returns all stored entities in this pool.
func (p Pool[K, V]) All() []PoolEntity[K, V] {
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
// the first inserted element.
func (p Pool[K, V]) Head() (value V, ok bool) {
	if p.states[stateUsed].size == 0 {
		return value, false
	}
	e := p.poolEntities[p.states[stateUsed].head]
	return e.Entity(), true
}

// sliceIndexForEntity returns a slice index which hosts the next entity to be added to the list.
// This index is invalid if there are no available slots or ejection could not be performed.
// If the valid index is returned then it is guaranteed that it corresponds to a free list head.
// Thus when filled with a new entity a switchState must be applied.
//
// The first boolean return value (hasAvailableSlot) says whether pool has an available slot.
// Pool goes out of available slots if it is full and no ejection is set.
//
// Ejection happens if there is no available slot, and there is an ejection mode set.
// If an ejection occurred, ejectedEntity holds the ejected entity.
func (p *Pool[K, V]) sliceIndexForEntity() (i EIndex, hasAvailableSlot bool, ejectedEntity V) {
	lruEject := func() (EIndex, bool, V) {
		// LRU ejection
		// the used head is the oldest entity, so we turn the used head to a free head here.
		invalidatedEntity := p.invalidateUsedHead()
		return p.states[stateFree].head, true, invalidatedEntity
	}

	if p.states[stateFree].size == 0 {
		// the free list is empty, so we are out of space, and we need to eject.
		switch p.ejectionMode {
		case NoEjection:
			// pool is set for no ejection, hence, no slice index is selected, abort immediately.
			return InvalidIndex, false, ejectedEntity
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
			invalidatedEntity := p.invalidateEntityAtIndex(randomIndex)
			return p.states[stateFree].head, true, invalidatedEntity
		case LRUEjection:
			// LRU ejection
			return lruEject()
		}
	}

	// returning the head of free list as the slice index for the next entity to be added
	return p.states[stateFree].head, true, ejectedEntity
}

// Size returns total number of entities that this list maintains.
func (p Pool[K, V]) Size() uint32 {
	return p.states[stateUsed].size
}

// getHeads returns entities corresponding to the used and free heads.
func (p Pool[K, V]) getHeads() (*poolEntity[K, V], *poolEntity[K, V]) {
	var usedHead, freeHead *poolEntity[K, V]
	if p.states[stateUsed].size != 0 {
		usedHead = &p.poolEntities[p.states[stateUsed].head]
	}

	if p.states[stateFree].size != 0 {
		freeHead = &p.poolEntities[p.states[stateFree].head]
	}

	return usedHead, freeHead
}

// getTails returns entities corresponding to the used and free tails.
func (p Pool[K, V]) getTails() (*poolEntity[K, V], *poolEntity[K, V]) {
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
// also removes the entity the invalidated head is presenting and appends the
// node represented by the used head to the tail of the free list.
func (p *Pool[K, V]) invalidateUsedHead() V {
	headSliceIndex := p.states[stateUsed].head
	return p.invalidateEntityAtIndex(headSliceIndex)
}

// Remove removes entity corresponding to given getSliceIndex from the list.
func (p *Pool[K, V]) Remove(sliceIndex EIndex) V {
	return p.invalidateEntityAtIndex(sliceIndex)
}

// invalidateEntityAtIndex invalidates the given getSliceIndex in the linked list by
// removing its corresponding linked-list node from the used linked list, and appending
// it to the tail of the free list. It also removes the entity that the invalidated node is presenting.
func (p *Pool[K, V]) invalidateEntityAtIndex(sliceIndex EIndex) V {
	invalidatedEntity := p.poolEntities[sliceIndex].value
	if invalidatedEntity == nil {
		panic(fmt.Sprintf("removing an entity from an empty slot with an index : %d", sliceIndex))
	}
	p.switchState(stateUsed, stateFree, sliceIndex)
	var zeroKey K
	var zeroValue V
	p.poolEntities[sliceIndex].key = zeroKey
	p.poolEntities[sliceIndex].value = zeroValue
	return invalidatedEntity
}

// isInvalidated returns true if linked-list node represented by getSliceIndex does not contain
// a valid entity.
func (p *Pool[K, V]) isInvalidated(sliceIndex EIndex) bool {
	var zeroKey K
	//var zeroValue V

	if p.poolEntities[sliceIndex].key != zeroKey {
		return false
	}

	if p.poolEntities[sliceIndex].value != nil { // Decide what to do with this comparison
		return false
	}

	return true
}

// switches state of an entity.
func (p *Pool[K, V]) switchState(stateFrom StateIndex, stateTo StateIndex, entityIndex EIndex) {
	// Remove from stateFrom list
	if p.states[stateFrom].size == 0 {
		panic("Removing an entity from an empty list")
	} else if p.states[stateFrom].size == 1 {
		p.states[stateFrom].head = InvalidIndex
		p.states[stateFrom].tail = InvalidIndex
	} else {
		node := p.poolEntities[entityIndex].node

		if entityIndex != p.states[stateFrom].head && entityIndex != p.states[stateFrom].tail {
			// links next and prev elements for non-head and non-tail element
			p.connect(node.prev, node.next)
		}

		if entityIndex == p.states[stateFrom].head {
			// moves head forward
			p.states[stateFrom].head = node.next
			p.poolEntities[p.states[stateFrom].head].node.prev = InvalidIndex
		}

		if entityIndex == p.states[stateFrom].tail {
			// moves tail backwards
			p.states[stateFrom].tail = node.prev
			p.poolEntities[p.states[stateFrom].tail].node.next = InvalidIndex
		}
	}
	p.states[stateFrom].size--

	// Add to stateTo list
	if p.states[stateTo].size == 0 {
		p.states[stateTo].head = entityIndex
		p.states[stateTo].tail = entityIndex
		p.poolEntities[p.states[stateTo].head].node.prev = InvalidIndex
		p.poolEntities[p.states[stateTo].tail].node.next = InvalidIndex
	} else {
		p.connect(p.states[stateTo].tail, entityIndex)
		p.states[stateTo].tail = entityIndex
		p.poolEntities[p.states[stateTo].tail].node.next = InvalidIndex
	}
	p.states[stateTo].size++
}
