package heropool

import (
	"fmt"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/rand"
)

type EjectionMode string

const (
	RandomEjection = EjectionMode("random-ejection")
	LRUEjection    = EjectionMode("lru-ejection")
	NoEjection     = EjectionMode("no-ejection")
)

// EIndex is data type representing an entity index in Pool.
type EIndex uint32

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
	logger zerolog.Logger
	// This size is a size of used linked list. As we will have it now as a part of
	// linked list it can be removed
	size         uint32
	free         state // keeps track of free slots.
	used         state // keeps track of allocated slots to cachedEntities.
	poolEntities []poolEntity
	ejectionMode EjectionMode
}

func NewHeroPool(sizeLimit uint32, ejectionMode EjectionMode, logger zerolog.Logger) *Pool {
	l := &Pool{
		free: state{
			head: poolIndex{index: 0},
			tail: poolIndex{index: 0},
			size: 0,
		},
		used: state{
			head: poolIndex{index: 0},
			tail: poolIndex{index: 0},
			size: 0,
		},
		poolEntities: make([]poolEntity, sizeLimit),
		ejectionMode: ejectionMode,
		logger:       logger,
	}

	l.initFreeEntities()

	return l
}

// initFreeEntities initializes the free double linked-list with the indices of all cached entity poolEntities.
func (p *Pool) initFreeEntities() {
	for i := 0; i < len(p.poolEntities); i++ {
		p.free.appendEntity(p, EIndex(i))
	}
}

// Add writes given entity into a poolEntity on the underlying entities linked-list.
//
// The boolean return value (slotAvailable) says whether pool has an available slot. Pool goes out of available slots if
// it is full and no ejection is set.
//
// If the pool has no available slots and an ejection is set, ejection occurs when adding a new entity.
// If an ejection occurred, ejectedEntity holds the ejected entity.

// done
func (p *Pool) Add(entityId flow.Identifier, entity flow.Entity, owner uint64) (
	entityIndex EIndex, slotAvailable bool, ejectedEntity flow.Entity) {
	entityIndex, slotAvailable, ejectedEntity = p.sliceIndexForEntity()
	if slotAvailable {
		p.poolEntities[entityIndex].entity = entity
		p.poolEntities[entityIndex].id = entityId
		p.poolEntities[entityIndex].owner = owner
		p.used.appendEntity(p, entityIndex)
	}

	return entityIndex, slotAvailable, ejectedEntity
}

// Get returns entity corresponding to the entity index from the underlying list.
func (p *Pool) Get(entityIndex EIndex) (flow.Identifier, flow.Entity, uint64) {
	return p.poolEntities[entityIndex].id, p.poolEntities[entityIndex].entity, p.poolEntities[entityIndex].owner
}

// All returns all stored entities in this pool.
func (p Pool) All() []PoolEntity {
	all := make([]PoolEntity, p.used.size)
	next := p.used.head

	for i := uint32(0); i < p.used.size; i++ {
		e := p.poolEntities[next.getSliceIndex()]
		all[i] = e.PoolEntity
		next = e.node.next
	}

	return all
}

// Head returns the head of used items. Assuming no ejection happened and pool never goes beyond limit, Head returns
// the first inserted element.
func (p Pool) Head() (flow.Entity, bool) {
	if p.used.size == 0 {
		return nil, false
	}
	e := p.poolEntities[p.used.head.getSliceIndex()]
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
	lruEject := func() (EIndex, bool, flow.Entity) {
		// LRU ejection
		// the used head is the oldest entity, so we turn the used head to a free head here.
		invalidatedEntity := p.invalidateUsedHead()
		return p.claimFreeHead(), true, invalidatedEntity
	}

	if p.free.head.isUndefined() {
		// the free list is empty, so we are out of space, and we need to eject.
		switch p.ejectionMode {
		case NoEjection:
			// pool is set for no ejection, hence, no slice index is selected, abort immediately.
			return 0, false, nil
		case RandomEjection:
			// we only eject randomly when the pool is full and random ejection is on.
			random, err := rand.Uint32n(p.size)
			if err != nil {
				p.logger.Fatal().Err(err).
					Msg("hero pool random ejection failed - falling back to LRU ejection")
				// fall back to LRU ejection only for this instance
				return lruEject()
			}
			randomIndex := EIndex(random)
			invalidatedEntity := p.invalidateEntityAtIndex(randomIndex)
			return p.claimFreeHead(), true, invalidatedEntity
		case LRUEjection:
			// LRU ejection
			return lruEject()
		}
	}

	// claiming the head of free list as the slice index for the next entity to be added
	return p.claimFreeHead(), true, nil
}

// Size returns total number of entities that this list maintains.
func (p Pool) Size() uint32 {
	return p.used.size
}

// getHeads returns entities corresponding to the used and free heads.
func (p *Pool) getHeads() (*poolEntity, *poolEntity) {
	var usedHead, freeHead *poolEntity

	if p.used.size != 0 {
		usedHead = &p.poolEntities[p.used.head.getSliceIndex()]
	}

	if p.free.size != 0 {
		freeHead = &p.poolEntities[p.free.head.getSliceIndex()]
	}

	return usedHead, freeHead
}

// getTails returns entities corresponding to the used and free tails.
func (p *Pool) getTails() (*poolEntity, *poolEntity) {
	var usedTail, freeTail *poolEntity
	if p.used.size != 0 {
		usedTail = &p.poolEntities[p.used.tail.getSliceIndex()]
	}

	if p.free.size != 0 {
		freeTail = &p.poolEntities[p.free.tail.getSliceIndex()]
	}

	return usedTail, freeTail
}

// connect links the prev and next nodes as the adjacent nodes in the double-linked list.
func (p *Pool) connect(prev poolIndex, next EIndex) {
	p.poolEntities[prev.getSliceIndex()].node.next.index = next
	p.poolEntities[next].node.prev = prev
}

// invalidateUsedHead moves current used head forward by one node. It
// also removes the entity the invalidated head is presenting and appends the
// node represented by the used head to the tail of the free list.
func (p *Pool) invalidateUsedHead() flow.Entity {
	headSliceIndex := p.used.head.getSliceIndex()
	return p.invalidateEntityAtIndex(headSliceIndex)
}

// claimFreeHead moves the free head forward, and returns the slice index of the
// old free head to host a new entity.
func (p *Pool) claimFreeHead() EIndex {
	oldFreeHeadIndex := p.free.head.getSliceIndex()

	if p.free.size == 0 {
		fmt.Println("debug shouldnt happen")
	}

	if p.free.size == 1 {
		p.free.size = 0
	}

	if p.free.size > 1 {
		p.free.head = p.poolEntities[oldFreeHeadIndex].node.next
		p.free.size = p.free.size - 1
	}

	return oldFreeHeadIndex
}

// Remove removes entity corresponding to given getSliceIndex from the list.
func (p *Pool) Remove(sliceIndex EIndex) flow.Entity {
	return p.invalidateEntityAtIndex(sliceIndex)
}

// invalidateEntityAtIndex invalidates the given getSliceIndex in the linked list by
// removing its corresponding linked-list node from the used linked list, and appending
// it to the tail of the free list. It also removes the entity that the invalidated node is presenting.
func (p *Pool) invalidateEntityAtIndex(sliceIndex EIndex) flow.Entity {
	poolEntity := p.poolEntities[sliceIndex]
	invalidatedEntity := poolEntity.entity
	p.used.removeEntity(p, sliceIndex)
	p.poolEntities[sliceIndex].id = flow.ZeroID
	p.poolEntities[sliceIndex].entity = nil
	p.free.appendEntity(p, EIndex(sliceIndex))

	return invalidatedEntity
}

// isInvalidated returns true if linked-list node represented by getSliceIndex does not contain
// a valid entity.
func (p *Pool) isInvalidated(sliceIndex EIndex) bool {
	if p.poolEntities[sliceIndex].id != flow.ZeroID {
		return false
	}

	if p.poolEntities[sliceIndex].entity != nil {
		return false
	}

	return true
}
