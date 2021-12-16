package arraylinkedlist

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

// EIndex is data type representing an entity index in EntityDoubleLinkedList.
type EIndex uint32

// doubleLinkedListPointer represents a slice-based linked list pointer. Instead of pointing
// to a memory address, this pointer points to a slice index.
//
// Note: an "undefined" (i.e., nil) notion for this doubleLinkedListPointer corresponds to the
// value of uint32(0). Hence, legit "pointer" values start from uint32(1).
// doubleLinkedListPointer also furnished with methods to convert a "pointer" value to
// a slice index, and reverse.
type doubleLinkedListPointer struct {
	pointerValue uint32
}

// isUndefined returns true if this pointer is set to its zero value. An undefined
// slice-based pointer is equivalent to a nil address-based one.
func (d doubleLinkedListPointer) isUndefined() bool {
	return d.pointerValue == uint32(0)
}

// setUndefined sets sliced-based pointer to its undefined (i.e., nil equivalent) value.
func (d *doubleLinkedListPointer) setUndefined() {
	d.pointerValue = uint32(0)
}

// sliceIndex returns the slice-index equivalent of the pointer.
func (d doubleLinkedListPointer) sliceIndex() EIndex {
	return EIndex(d.pointerValue) - 1
}

// setPointer converts the input slice-based index into a slice-based pointer and
// sets the underlying pointer.
func (d *doubleLinkedListPointer) setPointer(sliceIndex EIndex) {
	d.pointerValue = uint32(sliceIndex + 1)
}

// doubleLinkedListNode represents a slice-based double linked list node that
// consists of a next and previous pointer.
type doubleLinkedListNode struct {
	next doubleLinkedListPointer
	prev doubleLinkedListPointer
}

// doubleLinkedList represents a double linked list by its head and tail pointers.
type doubleLinkedList struct {
	head doubleLinkedListPointer
	tail doubleLinkedListPointer
}

func newDoubleLinkedList() *doubleLinkedList {
	return &doubleLinkedList{
		head: doubleLinkedListPointer{pointerValue: 0},
		tail: doubleLinkedListPointer{pointerValue: 0},
	}
}

// cachedEntity represents a cached entity that is maintained as a double linked list node.
type cachedEntity struct {
	owner  uint64
	node   doubleLinkedListNode
	id     flow.Identifier
	entity flow.Entity
}

type EntityDoubleLinkedList struct {
	total        uint32
	free         *doubleLinkedList // keeps track of free slots.
	used         *doubleLinkedList // keeps track of allocated slots to cachedEntities.
	values       []cachedEntity
	ejectionMode EjectionMode
}

func NewEntityList(limit uint32, ejectionMode EjectionMode) *EntityDoubleLinkedList {
	l := &EntityDoubleLinkedList{
		free:         newDoubleLinkedList(),
		used:         newDoubleLinkedList(),
		values:       make([]cachedEntity, limit),
		ejectionMode: ejectionMode,
	}

	l.initFreeEntities(limit)

	return l
}

// initFreeEntities initializes the free double linked-list with all slice indices in the range of
// [0, limit). In other words, all slice indices in cachedEntity are marked as free indices.
func (e *EntityDoubleLinkedList) initFreeEntities(limit uint32) {
	e.free.head.setPointer(0)
	e.free.tail.setPointer(0)

	for i := uint32(1); i < limit; i++ {
		// appends slice index i to tail of free linked list
		e.link(e.free.tail, EIndex(i))
		// and updates its tail
		e.free.tail.setPointer(EIndex(i))
	}
}

// Add writes given entity into a cachedEntity on the underlying entities linked-list. Return value is
// the index at which given entity is written on entities linked-list so that it can be accessed directly later.
func (e *EntityDoubleLinkedList) Add(entityId flow.Identifier, entity flow.Entity, owner uint64) EIndex {
	entityIndex := e.sliceIndexForEntity()
	e.values[entityIndex].entity = entity
	e.values[entityIndex].id = entityId
	e.values[entityIndex].owner = owner
	e.values[entityIndex].node.next.setUndefined()
	e.values[entityIndex].node.prev.setUndefined()

	if e.used.head.isUndefined() {
		// used list is empty, hence setting head of used list to current entityIndex.
		e.used.head.setPointer(entityIndex)
		e.values[e.used.head.sliceIndex()].node.prev.setUndefined()
	}

	if !e.used.tail.isUndefined() {
		// links new entity to the tail
		e.link(e.used.tail, entityIndex)
	}

	// since we are appending to the used list, entityIndex also acts as tail of the list.
	e.used.tail.setPointer(entityIndex)

	e.total++
	return entityIndex
}

// Get returns entity corresponding to the entity index from the underlying list.
func (e EntityDoubleLinkedList) Get(entityIndex EIndex) (flow.Identifier, flow.Entity, uint64) {
	return e.values[entityIndex].id, e.values[entityIndex].entity, e.values[entityIndex].owner
}

// sliceIndexForEntity returns a slice index which hosts the next entity to be added to the list.
func (e *EntityDoubleLinkedList) sliceIndexForEntity() EIndex {
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
func (e EntityDoubleLinkedList) Size() uint32 {
	return e.total
}

// getHeads returns entities corresponding to the used and free heads.
func (e EntityDoubleLinkedList) getHeads() (*cachedEntity, *cachedEntity) {
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
func (e EntityDoubleLinkedList) getTails() (*cachedEntity, *cachedEntity) {
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
func (e *EntityDoubleLinkedList) link(prev doubleLinkedListPointer, next EIndex) {
	e.values[prev.sliceIndex()].node.next.setPointer(next)
	e.values[next].node.prev = prev
}

// invalidateUsedHead moves current used head forward by one node. It
// also removes the entity invalidated head is presenting, and appends the
// node representing by used head to the tail of free list.
func (e *EntityDoubleLinkedList) invalidateUsedHead() EIndex {
	headSliceIndex := e.used.head.sliceIndex()
	e.invalidateEntityAtIndex(headSliceIndex)

	return headSliceIndex
}

// invalidateRandomEntity invalidates a random node from used linked list, and appends
// it to the tail of free list. It also removes the entity that invalidated node is presenting,
func (e *EntityDoubleLinkedList) invalidateRandomEntity() EIndex {
	// inorder not to keep failing on finding a random valid node to invalidate,
	// we only try a limited number of times, and if we fail all, we invalidate the used head.
	var index = e.used.head.sliceIndex()

	for i := 0; i < maximumRandomTrials; i++ {
		candidate := EIndex(rand.Uint32() % e.total)
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
func (e *EntityDoubleLinkedList) claimFreeHead() EIndex {
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
func (e *EntityDoubleLinkedList) Rem(sliceIndex EIndex) {
	e.invalidateEntityAtIndex(sliceIndex)
}

// invalidateRandomEntity invalidates the given sliceIndex in the linked list by
// removing its corresponding linked-list node from used linked list, and appending
// it to the tail of free list. It also removes the entity that invalidated node is presenting,
func (e *EntityDoubleLinkedList) invalidateEntityAtIndex(sliceIndex EIndex) {
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
	e.total--
}

// appendToFreeList appends linked-list node represented by sliceIndex to tail of free list.
func (e *EntityDoubleLinkedList) appendToFreeList(sliceIndex EIndex) {
	if e.free.head.isUndefined() {
		// free list is empty
		e.free.head.setPointer(sliceIndex)
		e.free.tail.setPointer(sliceIndex)
		return
	}

	// appends to the tail, and updates the tail
	e.link(e.free.tail, sliceIndex)
	e.free.tail.setPointer(sliceIndex)
}

// isInvalidate returns true if linked-list node represented by sliceIndex does not contain
// a valid entity.
func (e EntityDoubleLinkedList) isInvalidated(sliceIndex EIndex) bool {
	if e.values[sliceIndex].id != flow.ZeroID {
		return false
	}

	if e.values[sliceIndex].entity != nil {
		return false
	}

	return true
}
