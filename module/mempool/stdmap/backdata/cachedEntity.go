package backdata

import (
	"math/rand"

	"github.com/onflow/flow-go/model/flow"
)

// number of entries we try to eject before giving up and ejecting LRU
const maximumRandomTrials = 10

type doubleLinkedListPointer struct {
	pointerValue uint32
}

func (d doubleLinkedListPointer) isUndefined() bool {
	return d.pointerValue == uint32(0)
}

func (d *doubleLinkedListPointer) setUndefined() {
	d.pointerValue = uint32(0)
}

func (d doubleLinkedListPointer) sliceIndex() uint32 {
	return uint32(d.pointerValue) - 1
}

func (d *doubleLinkedListPointer) setPointer(sliceIndex uint32) {
	d.pointerValue = sliceIndex + 1
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
	id     *flow.Identifier
	owner  uint64
	entity flow.Entity
}

type entityList struct {
	total        uint32
	free         *doubleLinkedList // keeps track of used entities in entities list
	used         *doubleLinkedList // keeps track of unused entities in entities list
	entities     []cachedEntity
	ejectionMode EjectionMode
}

func newEntityList(limit uint32, ejectionMode EjectionMode) *entityList {
	l := &entityList{
		free:         newDoubleLinkedList(),
		used:         newDoubleLinkedList(),
		entities:     make([]cachedEntity, limit),
		ejectionMode: ejectionMode,
	}

	l.initFreeEntities(limit)

	return l
}

func (e *entityList) initFreeEntities(limit uint32) {
	e.free.head.setPointer(0)
	e.free.tail.setPointer(0)

	for i := uint32(1); i < limit; i++ {
		e.link(e.free.tail, i)
		e.free.tail.setPointer(i)
	}

}

func (e *entityList) add(entityId flow.Identifier, entity flow.Entity, owner uint64) uint32 {
	entityIndex, ejection := e.valueIndexForEntity()
	e.entities[entityIndex].entity = entity
	e.entities[entityIndex].id = &entityId
	e.entities[entityIndex].owner = owner
	e.entities[entityIndex].next.setUndefined()
	e.entities[entityIndex].prev.setUndefined()

	if !ejection {
		e.total++
	}

	if e.used.head.isUndefined() {
		// sets head
		e.used.head.setPointer(entityIndex)
		e.entities[e.used.head.sliceIndex()].prev.setUndefined()
	}

	if !e.used.tail.isUndefined() {
		// links new entity to the tail
		e.link(e.used.tail, entityIndex)
	}

	e.used.tail.setPointer(entityIndex)
	return entityIndex
}

func (e entityList) get(entityIndex uint32) (flow.Identifier, flow.Entity, uint64) {
	return *e.entities[entityIndex].id, e.entities[entityIndex].entity, e.entities[entityIndex].owner
}

func (e entityList) valueIndexForEntity() (uint32, bool) {
	limit := uint32(len(e.entities))
	if e.total < limit {
		// we are not over the limit yet.
		return e.total, false
	} else {
		// array back data is full
		if e.ejectionMode == RandomEjection {
			// ejecting a random entity

			return e.invalidateRandomEntity(), true
		} else {
			// ejecting eldest entity
			return e.invalidateHead(), true
		}
	}
}

func (e entityList) size() uint32 {
	return e.total
}

func (e entityList) getHead() *cachedEntity {
	return &e.entities[e.used.head.sliceIndex()]
}

func (e entityList) getTail() *cachedEntity {
	return &e.entities[e.used.tail.sliceIndex()]
}

func (e *entityList) link(prev doubleLinkedListPointer, next uint32) {
	e.entities[prev.sliceIndex()].next.setPointer(next)
	e.entities[next].prev = prev
}

func (e *entityList) invalidateHead() uint32 {
	headSliceIndex := e.used.head.sliceIndex()
	e.invalidateEntityAtIndex(headSliceIndex)

	return headSliceIndex
}

func (e *entityList) invalidateRandomEntity() uint32 {
	var index = e.used.head.sliceIndex()

	for i := 0; i < maximumRandomTrials; i++ {
		candidate := rand.Uint32() % e.total
		if !e.isInvalidated(candidate) {
			// found an invalidated entity
			index = candidate
			break
		}
	}

	e.invalidateEntityAtIndex(index)
	return index
}

func (e *entityList) invalidateEntityAtIndex(sliceIndex uint32) {
	prev := e.entities[sliceIndex].prev
	next := e.entities[sliceIndex].next

	if sliceIndex != e.used.head.sliceIndex() && sliceIndex != e.used.tail.sliceIndex() {
		// links next and prev elements for non-head and non-tail element
		e.link(prev, next.sliceIndex())
	}

	if sliceIndex == e.used.head.sliceIndex() {
		// moves head forward
		e.used.head = e.getHead().next
		// new head should point head to an undefined prev,
		// but we first check if list is not empty, i.e.,
		// head itself is not undefined.
		if !e.used.head.isUndefined() {
			e.getHead().prev.setUndefined()
		}
	}

	if sliceIndex == e.used.tail.sliceIndex() {
		// moves tail backward
		e.used.tail = e.getTail().prev
		// new head should point tail to an undefined next,
		// but we first check if list is not empty, i.e.,
		// tail itself is not undefined.
		if !e.used.tail.isUndefined() {
			e.getTail().next.setUndefined()
		}
	}

	// invalidates entity and adds it to free entities.
	e.entities[sliceIndex].id = nil
	e.entities[sliceIndex].next.setUndefined()
	e.entities[sliceIndex].prev.setUndefined()
	e.link(e.free.tail, sliceIndex)

	// decrements size
	e.total--
}

func (e entityList) isInvalidated(sliceIndex uint32) bool {
	if e.entities[sliceIndex].id != nil {
		return false
	}

	return true
}
