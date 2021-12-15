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

// VIndex is data type representing an entity index in EntityDoubleLinkedList.
type VIndex uint32

type doubleLinkedListPointer struct {
	pointerValue uint32
}

func (d doubleLinkedListPointer) isUndefined() bool {
	return d.pointerValue == uint32(0)
}

func (d *doubleLinkedListPointer) setUndefined() {
	d.pointerValue = uint32(0)
}

func (d doubleLinkedListPointer) sliceIndex() VIndex {
	return VIndex(d.pointerValue) - 1
}

func (d *doubleLinkedListPointer) setPointer(sliceIndex VIndex) {
	d.pointerValue = uint32(sliceIndex + 1)
}

type doubleLinkedListNode struct {
	next doubleLinkedListPointer
	prev doubleLinkedListPointer
}

type doubleLinkedList struct {
	head doubleLinkedListPointer // index of the head of linked list in entities slice.
	tail doubleLinkedListPointer // index of the tail of linked list in entities slice.
}

func newDoubleLinkedList() *doubleLinkedList {
	return &doubleLinkedList{
		head: doubleLinkedListPointer{pointerValue: 0},
		tail: doubleLinkedListPointer{pointerValue: 0},
	}
}

type cachedEntity struct {
	doubleLinkedListNode
	id     flow.Identifier
	owner  uint64
	entity flow.Entity
}

type EntityDoubleLinkedList struct {
	total        uint32
	free         *doubleLinkedList // keeps track of used entities in entities list
	used         *doubleLinkedList // keeps track of unused entities in entities list
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

func (e *EntityDoubleLinkedList) initFreeEntities(limit uint32) {
	e.free.head.setPointer(0)
	e.free.tail.setPointer(0)

	for i := uint32(1); i < limit; i++ {
		e.link(e.free.tail, VIndex(i))
		e.free.tail.setPointer(VIndex(i))
	}

}

func (e *EntityDoubleLinkedList) Add(entityId flow.Identifier, entity flow.Entity, owner uint64) VIndex {
	entityIndex := e.sliceIndexForEntity()
	e.values[entityIndex].entity = entity
	e.values[entityIndex].id = entityId
	e.values[entityIndex].owner = owner
	e.values[entityIndex].next.setUndefined()
	e.values[entityIndex].prev.setUndefined()

	if e.used.head.isUndefined() {
		// sets head
		e.used.head.setPointer(entityIndex)
		e.values[e.used.head.sliceIndex()].prev.setUndefined()
	}

	if !e.used.tail.isUndefined() {
		// links new entity to the tail
		e.link(e.used.tail, entityIndex)
	}

	e.used.tail.setPointer(entityIndex)

	e.total++
	return entityIndex
}

func (e EntityDoubleLinkedList) Get(entityIndex VIndex) (flow.Identifier, flow.Entity, uint64) {
	return e.values[entityIndex].id, e.values[entityIndex].entity, e.values[entityIndex].owner
}

func (e *EntityDoubleLinkedList) sliceIndexForEntity() VIndex {
	if e.free.head.isUndefined() {
		// we are at limit
		// array back data is full
		if e.ejectionMode == RandomEjection {
			// ejecting a random entity
			e.invalidateRandomEntity()
		} else {
			// turning used head to a free head.
			e.invalidateHead()
		}
	}

	return e.claimFreeHead()
}

func (e EntityDoubleLinkedList) Size() uint32 {
	return e.total
}

// used, free
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

func (e *EntityDoubleLinkedList) link(prev doubleLinkedListPointer, next VIndex) {
	e.values[prev.sliceIndex()].next.setPointer(next)
	e.values[next].prev = prev
}

func (e *EntityDoubleLinkedList) invalidateHead() VIndex {
	headSliceIndex := e.used.head.sliceIndex()
	e.invalidateEntityAtIndex(headSliceIndex)

	return headSliceIndex
}

func (e *EntityDoubleLinkedList) invalidateRandomEntity() VIndex {
	var index = e.used.head.sliceIndex()

	for i := 0; i < maximumRandomTrials; i++ {
		candidate := VIndex(rand.Uint32() % e.total)
		if !e.isInvalidated(candidate) {
			// found an invalidated entity
			index = candidate
			break
		}
	}

	e.invalidateEntityAtIndex(index)
	return index
}

func (e *EntityDoubleLinkedList) claimFreeHead() VIndex {
	oldFreeHeadIndex := e.free.head.sliceIndex()
	// moves head forward
	e.free.head = e.values[oldFreeHeadIndex].next
	// new head should point head to an undefined prev,
	// but we first check if list is not empty, i.e.,
	// head itself is not undefined.
	if !e.free.head.isUndefined() {
		e.values[e.free.head.sliceIndex()].prev.setUndefined()
	}

	// also we check if old head and tail aligned so to update
	// tail as well
	if e.free.tail.sliceIndex() == oldFreeHeadIndex {
		e.free.tail.setUndefined()
	}

	// clears pointers of claimed head
	e.values[oldFreeHeadIndex].next.setUndefined()
	e.values[oldFreeHeadIndex].prev.setUndefined()

	return oldFreeHeadIndex
}

func (e *EntityDoubleLinkedList) Rem(sliceIndex VIndex) {
	e.invalidateEntityAtIndex(sliceIndex)
}

func (e *EntityDoubleLinkedList) invalidateEntityAtIndex(sliceIndex VIndex) {
	prev := e.values[sliceIndex].prev
	next := e.values[sliceIndex].next

	if sliceIndex != e.used.head.sliceIndex() && sliceIndex != e.used.tail.sliceIndex() {
		// links next and prev elements for non-head and non-tail element
		e.link(prev, next.sliceIndex())
	}

	if sliceIndex == e.used.head.sliceIndex() {
		// moves head forward
		oldUsedHead, _ := e.getHeads()
		e.used.head = oldUsedHead.next
		// new head should point head to an undefined prev,
		// but we first check if list is not empty, i.e.,
		// head itself is not undefined.
		if !e.used.head.isUndefined() {
			usedHead, _ := e.getHeads()
			usedHead.prev.setUndefined()
		}
	}

	if sliceIndex == e.used.tail.sliceIndex() {
		// moves tail backward
		oldUsedTail, _ := e.getTails()
		e.used.tail = oldUsedTail.prev
		// new head should point tail to an undefined next,
		// but we first check if list is not empty, i.e.,
		// tail itself is not undefined.
		if !e.used.tail.isUndefined() {
			usedTail, _ := e.getTails()
			usedTail.next.setUndefined()
		}
	}

	// invalidates entity and adds it to free entities.
	e.values[sliceIndex].id = flow.ZeroID
	e.values[sliceIndex].entity = nil
	e.values[sliceIndex].next.setUndefined()
	e.values[sliceIndex].prev.setUndefined()

	e.appendToFreeList(sliceIndex)

	// decrements Size
	e.total--
}

func (e *EntityDoubleLinkedList) appendToFreeList(sliceIndex VIndex) {
	if e.free.head.isUndefined() {
		// free list is empty
		e.free.head.setPointer(sliceIndex)
		e.free.tail.setPointer(sliceIndex)
		return
	}

	e.link(e.free.tail, sliceIndex)
	e.free.tail.setPointer(sliceIndex)
}

func (e EntityDoubleLinkedList) isInvalidated(sliceIndex VIndex) bool {
	if e.values[sliceIndex].id != flow.ZeroID {
		return false
	}

	if e.values[sliceIndex].entity != nil {
		return false
	}

	return true
}
