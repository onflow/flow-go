package heropool

import (
	"math/rand"

	"github.com/onflow/flow-go/model/flow"
)

type EjectionMode string

const (
	RandomEjection = EjectionMode("random-ejection")
	LRUEjection    = EjectionMode("lru-ejection")
)

// number of entries we try to eject before giving up and ejecting LRU
const maximumRandomTrials = 10

// EIndex is data type representing an entity index in Pool.
type EIndex uint32

// poolEntity represents the data type that is maintained by
type poolEntity struct {
	// owner maintains an external reference to the key associated with this entity.
	// The key is maintained by the HeroCache, and entity is maintained by Pool.
	owner uint64

	// node keeps the link to the previous and next entities.
	// When this entity is allocated, the node maintains the connections it to the next and previous (used) pool entities.
	// When this entity is unallocated, the node maintains the connections to the next and previous unallocated (free) pool entities.
	node link

	// Identity associated with this entity.
	id flow.Identifier

	// Actual entity itself.
	entity flow.Entity
}

type Pool struct {
	size         uint32
	free         state // keeps track of free slots.
	used         state // keeps track of allocated slots to cachedEntities.
	poolEntities []poolEntity
	ejectionMode EjectionMode
}

func NewHeroPool(limit uint32, ejectionMode EjectionMode) *Pool {
	l := &Pool{
		free: state{
			head: poolIndex{index: 0},
			tail: poolIndex{index: 0},
		},
		used: state{
			head: poolIndex{index: 0},
			tail: poolIndex{index: 0},
		},
		poolEntities: make([]poolEntity, limit),
		ejectionMode: ejectionMode,
	}

	l.initFreeEntities()

	return l
}

// initFreeEntities initializes the free double linked-list with the indices of all cached entity poolEntities.
func (p *Pool) initFreeEntities() {
	p.free.head.setPoolIndex(0)
	p.free.tail.setPoolIndex(0)

	for i := 1; i < len(p.poolEntities); i++ {
		// appends slice index i to tail of free linked list
		p.connect(p.free.tail, EIndex(i))
		// and updates its tail
		p.free.tail.setPoolIndex(EIndex(i))
	}
}

// Add writes given entity into a poolEntity on the underlying entities linked-list. Return value is
// the index at which given entity is written on entities linked-list so that it can be accessed directly later.
func (p *Pool) Add(entityId flow.Identifier, entity flow.Entity, owner uint64) EIndex {
	entityIndex := p.sliceIndexForEntity()
	p.poolEntities[entityIndex].entity = entity
	p.poolEntities[entityIndex].id = entityId
	p.poolEntities[entityIndex].owner = owner
	p.poolEntities[entityIndex].node.next.setUndefined()
	p.poolEntities[entityIndex].node.prev.setUndefined()

	if p.used.head.isUndefined() {
		// used list is empty, hence setting head of used list to current entityIndex.
		p.used.head.setPoolIndex(entityIndex)
		p.poolEntities[p.used.head.sliceIndex()].node.prev.setUndefined()
	}

	if !p.used.tail.isUndefined() {
		// links new entity to the tail
		p.connect(p.used.tail, entityIndex)
	}

	// since we are appending to the used list, entityIndex also acts as tail of the list.
	p.used.tail.setPoolIndex(entityIndex)

	p.size++
	return entityIndex
}

// Get returns entity corresponding to the entity index from the underlying list.
func (p Pool) Get(entityIndex EIndex) (flow.Identifier, flow.Entity, uint64) {
	return p.poolEntities[entityIndex].id, p.poolEntities[entityIndex].entity, p.poolEntities[entityIndex].owner
}

// sliceIndexForEntity returns a slice index which hosts the next entity to be added to the list.
func (p *Pool) sliceIndexForEntity() EIndex {
	if p.free.head.isUndefined() {
		// the free list is empty, so we are out of space, and we need to eject.
		if p.ejectionMode == RandomEjection {
			// turning a random entity into a free head.
			p.invalidateRandomEntity()
		} else {
			// LRU ejection
			// the used head is the oldest entity, so we turn the used head to a free head here.
			p.invalidateUsedHead()
		}
	}

	// claiming the head of free list as the slice index for the next entity to be added
	return p.claimFreeHead()
}

// Size returns total number of entities that this list maintains.
func (p Pool) Size() uint32 {
	return p.size
}

// getHeads returns entities corresponding to the used and free heads.
func (p Pool) getHeads() (*poolEntity, *poolEntity) {
	var usedHead, freeHead *poolEntity
	if !p.used.head.isUndefined() {
		usedHead = &p.poolEntities[p.used.head.sliceIndex()]
	}

	if !p.free.head.isUndefined() {
		freeHead = &p.poolEntities[p.free.head.sliceIndex()]
	}

	return usedHead, freeHead
}

// getTails returns entities corresponding to the used and free tails.
func (p Pool) getTails() (*poolEntity, *poolEntity) {
	var usedTail, freeTail *poolEntity
	if !p.used.tail.isUndefined() {
		usedTail = &p.poolEntities[p.used.tail.sliceIndex()]
	}

	if !p.free.tail.isUndefined() {
		freeTail = &p.poolEntities[p.free.tail.sliceIndex()]
	}

	return usedTail, freeTail
}

// connect links the prev and next nodes as the adjacent nodes in the double-linked list.
func (p *Pool) connect(prev poolIndex, next EIndex) {
	p.poolEntities[prev.sliceIndex()].node.next.setPoolIndex(next)
	p.poolEntities[next].node.prev = prev
}

// invalidateUsedHead moves current used head forward by one node. It
// also removes the entity the invalidated head is presenting and appends the
// node represented by the used head to the tail of the free list.
func (p *Pool) invalidateUsedHead() EIndex {
	headSliceIndex := p.used.head.sliceIndex()
	p.invalidateEntityAtIndex(headSliceIndex)

	return headSliceIndex
}

// invalidateRandomEntity invalidates a random node from the used linked list, and appends it to the tail of the free list.
// It also removes the entity that the invalidated node is presenting.
func (p *Pool) invalidateRandomEntity() EIndex {
	// in order not to keep failing on finding a random valid node to invalidate,
	// we only try a limited number of times, and if we fail all, we invalidate the used head.
	var index = p.used.head.sliceIndex()

	for i := 0; i < maximumRandomTrials; i++ {
		candidate := EIndex(rand.Uint32() % p.size)
		if !p.isInvalidated(candidate) {
			// found a valid entity to invalidate
			index = candidate
			break
		}
	}

	p.invalidateEntityAtIndex(index)
	return index
}

// claimFreeHead moves the free head forward, and returns the slice index of the
// old free head to host a new entity.
func (p *Pool) claimFreeHead() EIndex {
	oldFreeHeadIndex := p.free.head.sliceIndex()
	// moves head forward
	p.free.head = p.poolEntities[oldFreeHeadIndex].node.next
	// new head should point to an undefined prev,
	// but we first check if list is not empty, i.e.,
	// head itself is not undefined.
	if !p.free.head.isUndefined() {
		p.poolEntities[p.free.head.sliceIndex()].node.prev.setUndefined()
	}

	// also, we check if the old head and tail are aligned and, if so, update the
	// tail as well. This happens when we claim the only existing
	// node of the free list.
	if p.free.tail.sliceIndex() == oldFreeHeadIndex {
		p.free.tail.setUndefined()
	}

	// clears pointers of claimed head
	p.poolEntities[oldFreeHeadIndex].node.next.setUndefined()
	p.poolEntities[oldFreeHeadIndex].node.prev.setUndefined()

	return oldFreeHeadIndex
}

// Rem removes entity corresponding to given sliceIndex from the list.
func (p *Pool) Rem(sliceIndex EIndex) {
	p.invalidateEntityAtIndex(sliceIndex)
}

// invalidateEntityAtIndex invalidates the given sliceIndex in the linked list by
// removing its corresponding linked-list node from the used linked list, and appending
// it to the tail of the free list. It also removes the entity that the invalidated node is presenting.
func (p *Pool) invalidateEntityAtIndex(sliceIndex EIndex) {
	prev := p.poolEntities[sliceIndex].node.prev
	next := p.poolEntities[sliceIndex].node.next

	if sliceIndex != p.used.head.sliceIndex() && sliceIndex != p.used.tail.sliceIndex() {
		// links next and prev elements for non-head and non-tail element
		p.connect(prev, next.sliceIndex())
	}

	if sliceIndex == p.used.head.sliceIndex() {
		// invalidating used head
		// moves head forward
		oldUsedHead, _ := p.getHeads()
		p.used.head = oldUsedHead.node.next
		// new head should point to an undefined prev,
		// but we first check if list is not empty, i.e.,
		// head itself is not undefined.
		if !p.used.head.isUndefined() {
			usedHead, _ := p.getHeads()
			usedHead.node.prev.setUndefined()
		}
	}

	if sliceIndex == p.used.tail.sliceIndex() {
		// invalidating used tail
		// moves tail backward
		oldUsedTail, _ := p.getTails()
		p.used.tail = oldUsedTail.node.prev
		// new head should point tail to an undefined next,
		// but we first check if list is not empty, i.e.,
		// tail itself is not undefined.
		if !p.used.tail.isUndefined() {
			usedTail, _ := p.getTails()
			usedTail.node.next.setUndefined()
		}
	}

	// invalidates entity and adds it to free entities.
	p.poolEntities[sliceIndex].id = flow.ZeroID
	p.poolEntities[sliceIndex].entity = nil
	p.poolEntities[sliceIndex].node.next.setUndefined()
	p.poolEntities[sliceIndex].node.prev.setUndefined()

	p.appendToFreeList(sliceIndex)

	// decrements Size
	p.size--
}

// appendToFreeList appends linked-list node represented by sliceIndex to tail of free list.
func (p *Pool) appendToFreeList(sliceIndex EIndex) {
	if p.free.head.isUndefined() {
		// free list is empty
		p.free.head.setPoolIndex(sliceIndex)
		p.free.tail.setPoolIndex(sliceIndex)
		return
	}

	// appends to the tail, and updates the tail
	p.connect(p.free.tail, sliceIndex)
	p.free.tail.setPoolIndex(sliceIndex)
}

// isInvalidated returns true if linked-list node represented by sliceIndex does not contain
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
