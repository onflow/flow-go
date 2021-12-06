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
	id     flow.Identifier
	owner  uint64
	entity flow.Entity
}

type entityList struct {
	total             uint64
	freeEntities      *doubleLinkedList // keeps track of used entities in entities list
	allocatedEntities *doubleLinkedList // keeps track of unused entities in entities list
	entities          []cachedEntity
	ejectionMode      EjectionMode
}

func newEntityList(limit uint64, ejectionMode EjectionMode) *entityList {
	return &entityList{
		freeEntities:      newDoubleLinkedList(),
		allocatedEntities: newDoubleLinkedList(),
		entities:          make([]cachedEntity, limit),
		ejectionMode:      ejectionMode,
	}
}

func (e *entityList) add(entityId flow.Identifier, entity flow.Entity, owner uint64) uint32 {
	entityIndex, ejection := e.valueIndexForEntity()
	e.entities[entityIndex].entity = entity
	e.entities[entityIndex].id = entityId
	e.entities[entityIndex].owner = owner
	e.entities[entityIndex].next.setUndefined()
	e.entities[entityIndex].prev.setUndefined()

	if !ejection {
		e.total++
	}

	if e.allocatedEntities.head.isUndefined() {
		// sets head
		e.allocatedEntities.head.setPointer(entityIndex)
		e.entities[e.allocatedEntities.head.sliceIndex()].prev.setUndefined()
	}

	if !e.allocatedEntities.tail.isUndefined() {
		// links new entity to the tail
		e.link(e.allocatedEntities.tail, entityIndex)
	}

	e.allocatedEntities.tail.setPointer(entityIndex)
	return entityIndex
}

func (e entityList) get(entityIndex uint32) (flow.Identifier, flow.Entity, uint64) {
	return e.entities[entityIndex].id, e.entities[entityIndex].entity, e.entities[entityIndex].owner
}

func (e entityList) valueIndexForEntity() (uint32, bool) {
	limit := uint64(len(e.entities))
	if e.total < limit {
		// we are not over the limit yet.
		return uint32(e.total), false
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

func (e entityList) size() uint64 {
	return e.total
}

func (e entityList) getHead() *cachedEntity {
	return &e.entities[e.allocatedEntities.head.sliceIndex()]
}

func (e entityList) getTail() *cachedEntity {
	return &e.entities[e.allocatedEntities.tail.sliceIndex()]
}

func (e *entityList) link(prev doubleLinkedListPointer, next uint32) {
	e.entities[prev.sliceIndex()].next.setPointer(next)
	e.entities[next].prev = prev
}

func (e *entityList) invalidateHead() uint32 {
	headSliceIndex := e.allocatedEntities.head.sliceIndex()
	e.invalidateEntityAtIndex(headSliceIndex)

	return headSliceIndex
}

func (e *entityList) invalidateRandomEntity() uint32 {
	var index = e.allocatedEntities.head.sliceIndex()

	for i := 0; i < maximumRandomTrials; i++ {
		candidate := uint32(rand.Uint64() % e.total)
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

	if sliceIndex != e.allocatedEntities.head.sliceIndex() && sliceIndex != e.allocatedEntities.tail.sliceIndex() {
		// links next and prev elements for non-head and non-tail element
		e.link(prev, next.sliceIndex())
	}

	if sliceIndex == e.allocatedEntities.head.sliceIndex() {
		// moves head forward
		e.allocatedEntities.head = e.getHead().next
		// new head should point head to an undefined prev,
		// but we first check if list is not empty, i.e.,
		// head itself is not undefined.
		if !e.allocatedEntities.head.isUndefined() {
			e.getHead().prev.setUndefined()
		}
	}

	if sliceIndex == e.allocatedEntities.tail.sliceIndex() {
		// moves tail backward
		e.allocatedEntities.tail = e.getTail().prev
		// new head should point tail to an undefined next,
		// but we first check if list is not empty, i.e.,
		// tail itself is not undefined.
		if !e.allocatedEntities.tail.isUndefined() {
			e.getTail().next.setUndefined()
		}
	}

	// invalidates entity
	e.entities[sliceIndex].id = flow.ZeroID
	e.entities[sliceIndex].next.setUndefined()
	e.entities[sliceIndex].prev.setUndefined()

	// decrements size
	e.total--
}

func (e entityList) isInvalidated(sliceIndex uint32) bool {
	if e.entities[sliceIndex].id != flow.ZeroID {
		return false
	}

	// if either next or prev pointers are linked, the entity is not invalidated.
	if !e.entities[sliceIndex].next.isUndefined() || !e.entities[sliceIndex].prev.isUndefined() {
		return false
	}

	return true
}
