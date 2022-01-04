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

// cachedEntity represents a cached entity that is maintained as a double linked list node.
type cachedEntity struct {
	owner  uint64
	node   link
	id     flow.Identifier
	entity flow.Entity
}

type Pool struct {
	size         uint32
	free         *state // keeps track of free slots.
	used         *state // keeps track of allocated slots to cachedEntities.
	values       []cachedEntity
	ejectionMode EjectionMode
}

func NewPool(limit uint32, ejectionMode EjectionMode) *Pool {
	l := &Pool{
		free:         newState(),
		used:         newState(),
		values:       make([]cachedEntity, limit),
		ejectionMode: ejectionMode,
	}

	l.initFreeEntities(limit)

	return l
}

// initFreeEntities initializes the free double linked-list with all slice indices in the range of
// [0, limit). In other words, all slice indices in cachedEntity are marked as free indices.
func (e *Pool) initFreeEntities(limit uint32) {
	e.free.head.setPoolIndex(0)
	e.free.tail.setPoolIndex(0)

	for i := uint32(1); i < limit; i++ {
		// appends slice index i to tail of free linked list
		e.link(e.free.tail, EIndex(i))
		// and updates its tail
		e.free.tail.setPoolIndex(EIndex(i))
	}
}

// Add writes given entity into a cachedEntity on the underlying entities linked-list. Return value is
// the index at which given entity is written on entities linked-list so that it can be accessed directly later.
func (e *Pool) Add(entityId flow.Identifier, entity flow.Entity, owner uint64) EIndex {
	entityIndex := e.sliceIndexForEntity()
	e.values[entityIndex].entity = entity
	e.values[entityIndex].id = entityId
	e.values[entityIndex].owner = owner
	e.values[entityIndex].node.next.setUndefined()
	e.values[entityIndex].node.prev.setUndefined()

	if e.used.head.isUndefined() {
		// used list is empty, hence setting head of used list to current entityIndex.
		e.used.head.setPoolIndex(entityIndex)
		e.values[e.used.head.sliceIndex()].node.prev.setUndefined()
	}

	if !e.used.tail.isUndefined() {
		// links new entity to the tail
		e.link(e.used.tail, entityIndex)
	}

	// since we are appending to the used list, entityIndex also acts as tail of the list.
	e.used.tail.setPoolIndex(entityIndex)

	e.size++
	return entityIndex
}

// Get returns entity corresponding to the entity index from the underlying list.
func (e Pool) Get(entityIndex EIndex) (flow.Identifier, flow.Entity, uint64) {
	return e.values[entityIndex].id, e.values[entityIndex].entity, e.values[entityIndex].owner
}

// sliceIndexForEntity returns a slice index which hosts the next entity to be added to the list.
func (e *Pool) sliceIndexForEntity() EIndex {
	if e.free.head.isUndefined() {
		// free list is empty, hence, we are out of space, we
		// need to eject.
		if e.ejectionMode == RandomEjection {
			// turning a random entity into a free head.
			e.invalidateRandomEntity()
		} else {
			// LRU ejection
			// used head is the oldest entity, hence,
			// turning the used head to a free head.
			e.invalidateUsedHead()
		}
	}

	// claiming the head of free list as the slice index for the next entity to be added
	return e.claimFreeHead()
}

// Size returns total number of entities that this list maintains.
func (e Pool) Size() uint32 {
	return e.size
}

// getHeads returns entities corresponding to the used and free heads.
func (e Pool) getHeads() (*cachedEntity, *cachedEntity) {
	var usedHead, freeHead *cachedEntity
	if !e.used.head.isUndefined() {
		usedHead = &e.values[e.used.head.sliceIndex()]
	}

	if !e.free.head.isUndefined() {
		freeHead = &e.values[e.free.head.sliceIndex()]
	}

	return usedHead, freeHead
}

// getTails returns entities corresponding to the used and free tails.
func (e Pool) getTails() (*cachedEntity, *cachedEntity) {
	var usedTail, freeTail *cachedEntity
	if !e.used.tail.isUndefined() {
		usedTail = &e.values[e.used.tail.sliceIndex()]
	}

	if !e.free.tail.isUndefined() {
		freeTail = &e.values[e.free.tail.sliceIndex()]
	}

	return usedTail, freeTail
}

// link connects the prev and next nodes as the adjacent nodes in the double-linked list.
func (e *Pool) link(prev poolIndex, next EIndex) {
	e.values[prev.sliceIndex()].node.next.setPoolIndex(next)
	e.values[next].node.prev = prev
}

// invalidateUsedHead moves current used head forward by one node. It
// also removes the entity invalidated head is presenting, and appends the
// node representing by used head to the tail of free list.
func (e *Pool) invalidateUsedHead() EIndex {
	headSliceIndex := e.used.head.sliceIndex()
	e.invalidateEntityAtIndex(headSliceIndex)

	return headSliceIndex
}

// invalidateRandomEntity invalidates a random node from used linked list, and appends
// it to the tail of free list. It also removes the entity that invalidated node is presenting,
func (e *Pool) invalidateRandomEntity() EIndex {
	// inorder not to keep failing on finding a random valid node to invalidate,
	// we only try a limited number of times, and if we fail all, we invalidate the used head.
	var index = e.used.head.sliceIndex()

	for i := 0; i < maximumRandomTrials; i++ {
		candidate := EIndex(rand.Uint32() % e.size)
		if !e.isInvalidated(candidate) {
			// found a valid entity to invalidate
			index = candidate
			break
		}
	}

	e.invalidateEntityAtIndex(index)
	return index
}

// claimFreeHead moves the free head forward, and returns the slice index of the
// old free head to host a new entity.
func (e *Pool) claimFreeHead() EIndex {
	oldFreeHeadIndex := e.free.head.sliceIndex()
	// moves head forward
	e.free.head = e.values[oldFreeHeadIndex].node.next
	// new head should point to an undefined prev,
	// but we first check if list is not empty, i.e.,
	// head itself is not undefined.
	if !e.free.head.isUndefined() {
		e.values[e.free.head.sliceIndex()].node.prev.setUndefined()
	}

	// also, we check if old head and tail aligned so to update
	// tail as well. This happens when we claim the only existing
	// node of free list.
	if e.free.tail.sliceIndex() == oldFreeHeadIndex {
		e.free.tail.setUndefined()
	}

	// clears pointers of claimed head
	e.values[oldFreeHeadIndex].node.next.setUndefined()
	e.values[oldFreeHeadIndex].node.prev.setUndefined()

	return oldFreeHeadIndex
}

// Rem removes entity corresponding to given sliceIndex from the list.
func (e *Pool) Rem(sliceIndex EIndex) {
	e.invalidateEntityAtIndex(sliceIndex)
}

// invalidateRandomEntity invalidates the given sliceIndex in the linked list by
// removing its corresponding linked-list node from used linked list, and appending
// it to the tail of free list. It also removes the entity that invalidated node is presenting,
func (e *Pool) invalidateEntityAtIndex(sliceIndex EIndex) {
	prev := e.values[sliceIndex].node.prev
	next := e.values[sliceIndex].node.next

	if sliceIndex != e.used.head.sliceIndex() && sliceIndex != e.used.tail.sliceIndex() {
		// links next and prev elements for non-head and non-tail element
		e.link(prev, next.sliceIndex())
	}

	if sliceIndex == e.used.head.sliceIndex() {
		// invalidating used head
		// moves head forward
		oldUsedHead, _ := e.getHeads()
		e.used.head = oldUsedHead.node.next
		// new head should point to an undefined prev,
		// but we first check if list is not empty, i.e.,
		// head itself is not undefined.
		if !e.used.head.isUndefined() {
			usedHead, _ := e.getHeads()
			usedHead.node.prev.setUndefined()
		}
	}

	if sliceIndex == e.used.tail.sliceIndex() {
		// invalidating used tail
		// moves tail backward
		oldUsedTail, _ := e.getTails()
		e.used.tail = oldUsedTail.node.prev
		// new head should point tail to an undefined next,
		// but we first check if list is not empty, i.e.,
		// tail itself is not undefined.
		if !e.used.tail.isUndefined() {
			usedTail, _ := e.getTails()
			usedTail.node.next.setUndefined()
		}
	}

	// invalidates entity and adds it to free entities.
	e.values[sliceIndex].id = flow.ZeroID
	e.values[sliceIndex].entity = nil
	e.values[sliceIndex].node.next.setUndefined()
	e.values[sliceIndex].node.prev.setUndefined()

	e.appendToFreeList(sliceIndex)

	// decrements Size
	e.size--
}

// appendToFreeList appends linked-list node represented by sliceIndex to tail of free list.
func (e *Pool) appendToFreeList(sliceIndex EIndex) {
	if e.free.head.isUndefined() {
		// free list is empty
		e.free.head.setPoolIndex(sliceIndex)
		e.free.tail.setPoolIndex(sliceIndex)
		return
	}

	// appends to the tail, and updates the tail
	e.link(e.free.tail, sliceIndex)
	e.free.tail.setPoolIndex(sliceIndex)
}

// isInvalidate returns true if linked-list node represented by sliceIndex does not contain
// a valid entity.
func (e Pool) isInvalidated(sliceIndex EIndex) bool {
	if e.values[sliceIndex].id != flow.ZeroID {
		return false
	}

	if e.values[sliceIndex].entity != nil {
		return false
	}

	return true
}
