package heropool

import (
	"math"
	"math/rand"

	"github.com/onflow/flow-go/model/flow"
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

// InvalidIndex is used when a link doesnt point anywhere, in other words it is an equivalent of a nil adress.
const InvalidIndex EIndex = math.MaxUint32

// poolEntity represents the data type that is maintained by
type poolEntity struct {
	PoolEntity
	// owner maintains an external reference to the key associated with this entity.
	// The key is maintained by the HeroCache, and entity is maintained by Pool.
	owner uint64

	// node keeps the link to the previous and next entities.
	// When this entity is allocated, the node maintains the connections it to the next and previous (used) pool entities.
	// When this entity is unallocated, the node maintains the connections to the next and previous unallocated (free) pool entities.
	node link
}

type PoolEntity struct {
	// Identity associated with this entity.
	id flow.Identifier

	// Actual entity itself.
	entity flow.Entity
}

func (p PoolEntity) Id() flow.Identifier {
	return p.id
}

func (p PoolEntity) Entity() flow.Entity {
	return p.entity
}

type Pool struct {
	states       []state // keeps track of a slot's state
	poolEntities []poolEntity
	ejectionMode EjectionMode
}

func NewHeroPool(sizeLimit uint32, ejectionMode EjectionMode) *Pool {
	l := &Pool{
		//constructor for states make them invalid
		states:       NewStates(numberOfStates),
		poolEntities: make([]poolEntity, sizeLimit),
		ejectionMode: ejectionMode,
	}

	l.setDefaultNodeLinkValues()
	l.initFreeEntities()

	return l
}

// setDefaultNodeLinkValues sets nodes prev and next to InvalidIndex for all cached entities in poolEntities.
func (p *Pool) setDefaultNodeLinkValues() {
	for i := 0; i < len(p.poolEntities); i++ {
		p.poolEntities[i].node.next = InvalidIndex
		p.poolEntities[i].node.prev = InvalidIndex
	}
}

// initFreeEntities initializes the free double linked-list with the indices of all cached entity poolEntities.
func (p *Pool) initFreeEntities() {
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

// Add writes given entity into a poolEntity on the underlying entities linked-list.
//
// The boolean return value (slotAvailable) says whether pool has an available slot. Pool goes out of available slots if
// it is full and no ejection is set.
//
// If the pool has no available slots and an ejection is set, ejection occurs when adding a new entity.
// If an ejection occurred, ejectedEntity holds the ejected entity.
func (p *Pool) Add(entityId flow.Identifier, entity flow.Entity, owner uint64) (entityIndex EIndex, slotAvailable bool, ejectedEntity flow.Entity) {
	entityIndex, slotAvailable, ejectedEntity = p.sliceIndexForEntity()
	if slotAvailable {
		p.poolEntities[entityIndex].entity = entity
		p.poolEntities[entityIndex].id = entityId
		p.poolEntities[entityIndex].owner = owner
		p.changeState(stateFree, stateUsed, entityIndex)
	}

	return entityIndex, slotAvailable, ejectedEntity
}

// Get returns entity corresponding to the entity index from the underlying list.
func (p Pool) Get(entityIndex EIndex) (flow.Identifier, flow.Entity, uint64) {
	return p.poolEntities[entityIndex].id, p.poolEntities[entityIndex].entity, p.poolEntities[entityIndex].owner
}

// All returns all stored entities in this pool.
func (p Pool) All() []PoolEntity {
	all := make([]PoolEntity, p.states[stateUsed].size)
	next := p.states[stateUsed].head

	for i := uint32(0); i < p.states[stateUsed].size; i++ {
		e := p.poolEntities[next]
		all[i] = e.PoolEntity
		next = e.node.next
	}

	return all
}

// Head returns the head of states[stateUsed] items. Assuming no ejection happened and pool never goes beyond limit, Head returns
// the first inserted element.
func (p Pool) Head() (flow.Entity, bool) {

	if p.states[stateUsed].size == 0 {
		return nil, false
	}
	e := p.poolEntities[p.states[stateUsed].head]
	return e.Entity(), true
}

// sliceIndexForEntity returns a slice index which hosts the next entity to be added to the list.
//
// The first boolean return value (hasAvailableSlot) says whether pool has an available slot.
// Pool goes out of available slots if it is full and no ejection is set.
//
// Ejection happens if there is no available slot, and there is an ejection mode set.
// If an ejection occurred, ejectedEntity holds the ejected entity.
func (p *Pool) sliceIndexForEntity() (i EIndex, hasAvailableSlot bool, ejectedEntity flow.Entity) {
	if p.states[stateFree].size == 0 {
		// the states[stateFree] list is empty, so we are out of space, and we need to eject.
		switch p.ejectionMode {
		case NoEjection:
			// pool is set for no ejection, hence, no slice index is selected, abort immediately.
			return InvalidIndex, false, nil
		case LRUEjection:
			// LRU ejection
			// the states[stateUsed] head is the oldest entity, so we turn the states[stateUsed] head to a states[stateFree] head here.
			invalidatedEntity := p.invalidateUsedHead()
			return p.states[stateFree].head, true, invalidatedEntity
		case RandomEjection:
			// we only eject randomly when the pool is full and random ejection is on.
			randomIndex := EIndex(rand.Uint32() % p.states[stateUsed].size)
			invalidatedEntity := p.invalidateEntityAtIndex(randomIndex)
			return p.states[stateFree].head, true, invalidatedEntity
		}
	}

	// claiming the head of states[stateFree] list as the slice index for the next entity to be added
	return p.states[stateFree].head, true, nil
}

// Size returns total number of entities that this list maintains.
func (p Pool) Size() uint32 {
	return p.states[stateUsed].size
}

// getHeads returns entities corresponding to the used and free heads.
func (p Pool) getHeads() (*poolEntity, *poolEntity) {
	var usedHead, freeHead *poolEntity

	if p.states[stateUsed].size != 0 {
		usedHead = &p.poolEntities[p.states[stateUsed].head]
	}

	if p.states[stateFree].size != 0 {
		freeHead = &p.poolEntities[p.states[stateFree].head]
	}

	return usedHead, freeHead
}

// getTails returns entities corresponding to the states[stateUsed] and free tails.
func (p Pool) getTails() (*poolEntity, *poolEntity) {
	var usedTail, freeTail *poolEntity
	if p.states[stateUsed].size != 0 {
		usedTail = &p.poolEntities[p.states[stateUsed].tail]
	}
	if p.states[stateFree].size != 0 {
		freeTail = &p.poolEntities[p.states[stateFree].tail]
	}

	return usedTail, freeTail
}

// connect links the prev and next nodes as the adjacent nodes in the double-linked list.
func (p *Pool) connect(prev EIndex, next EIndex) {
	p.poolEntities[prev].node.next = next
	p.poolEntities[next].node.prev = prev
}

// invalidateUsedHead moves current states[stateUsed] head forward by one node. It
// also removes the entity the invalidated head is presenting and appends the
// node represented by the states[stateUsed] head to the tail of the states[stateFree] list.
func (p *Pool) invalidateUsedHead() flow.Entity {
	headSliceIndex := p.states[stateUsed].head
	return p.invalidateEntityAtIndex(headSliceIndex)
}

// Remove removes entity corresponding to given getSliceIndex from the list.
func (p *Pool) Remove(sliceIndex EIndex) flow.Entity {
	return p.invalidateEntityAtIndex(sliceIndex)
}

// invalidateEntityAtIndex invalidates the given getSliceIndex in the linked list by
// removing its corresponding linked-list node from the states[stateUsed] linked list, and appending
// it to the tail of the states[stateFree] list. It also removes the entity that the invalidated node is presenting.
func (p *Pool) invalidateEntityAtIndex(sliceIndex EIndex) flow.Entity {
	poolEntity := p.poolEntities[sliceIndex]
	invalidatedEntity := poolEntity.entity
	p.changeState(stateUsed, stateFree, sliceIndex)
	p.poolEntities[sliceIndex].id = flow.ZeroID
	p.poolEntities[sliceIndex].entity = nil
	return invalidatedEntity
}

// isInvalidated returns true if linked-list node represented by getSliceIndex does not contain
// a valid entity.
func (p Pool) isInvalidated(sliceIndex EIndex) bool {
	if p.poolEntities[sliceIndex].id != flow.ZeroID {
		return false
	}

	if p.poolEntities[sliceIndex].entity != nil {
		return false
	}

	return true
}

// utility method that removes an entity from one of the states.
// NOTE: a removed entity has to be added to another state.
func (p *Pool) removeEntity(stateType StateIndex, entityIndex EIndex) {
	if p.states[stateType].size == 0 {
		panic("Removing an entity from an empty list")
	}
	if p.states[stateType].size == 1 {
		// here set to InvalidIndex
		p.states[stateType].head = InvalidIndex
		p.states[stateType].tail = InvalidIndex
		p.states[stateType].size--
		p.poolEntities[entityIndex].node.next = InvalidIndex
		p.poolEntities[entityIndex].node.prev = InvalidIndex
		return
	}
	node := p.poolEntities[entityIndex].node

	if entityIndex != p.states[stateType].head && entityIndex != p.states[stateType].tail {
		// links next and prev elements for non-head and non-tail element
		p.connect(node.prev, node.next)
	}

	if entityIndex == p.states[stateType].head {
		// moves head forward
		p.states[stateType].head = node.next
		p.poolEntities[p.states[stateType].head].node.prev = InvalidIndex
	}

	if entityIndex == p.states[stateType].tail {
		// moves tail backwards
		p.states[stateType].tail = node.prev
		p.poolEntities[p.states[stateType].tail].node.next = InvalidIndex
	}
	p.states[stateType].size--
	p.poolEntities[entityIndex].node.next = InvalidIndex
	p.poolEntities[entityIndex].node.prev = InvalidIndex
}

// appends an entity to the tail of the state or creates a first element.
// NOTE: entity should not be in any list before this method is applied
func (p *Pool) appendEntity(stateType StateIndex, entityIndex EIndex) {

	if p.states[stateType].size == 0 {
		p.states[stateType].head = entityIndex
		p.states[stateType].tail = entityIndex
		p.poolEntities[p.states[stateType].head].node.prev = InvalidIndex
		p.poolEntities[p.states[stateType].tail].node.next = InvalidIndex
		p.states[stateType].size = 1
		return
	}
	p.connect(p.states[stateType].tail, entityIndex)
	p.states[stateType].size++
	p.states[stateType].tail = entityIndex
	p.poolEntities[p.states[stateType].tail].node.next = InvalidIndex
}

func (p *Pool) changeState(stateFrom StateIndex, stateTo StateIndex, entityIndex EIndex) error {
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
	return nil
}
