package backdata

import (
	"math/rand"

	"github.com/onflow/flow-go/model/flow"
)

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

type cachedEntity struct {
	doubleLinkedListNode
	id     flow.Identifier
	owner  uint64
	entity flow.Entity
}

type entityList struct {
	total        uint64
	head         doubleLinkedListPointer // index of the head of linked list in entities slice.
	tail         doubleLinkedListPointer // index of the tail of linked list in entities slice.
	entities     []cachedEntity
	ejectionMode EjectionMode
}

func newEntityList(limit uint64, ejectionMode EjectionMode) *entityList {
	return &entityList{
		head:         doubleLinkedListPointer{pointerValue: 0},
		tail:         doubleLinkedListPointer{pointerValue: 0},
		entities:     make([]cachedEntity, limit),
		ejectionMode: ejectionMode,
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

	if e.head.isUndefined() {
		// sets head
		e.head.setPointer(entityIndex)
		e.entities[e.head.sliceIndex()].prev.setUndefined()
	}

	if !e.tail.isUndefined() {
		// links new entity to the tail
		e.link(e.tail, entityIndex)
	}

	e.tail.setPointer(entityIndex)
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
			return uint32(rand.Uint64() % limit), true
		} else {
			// ejecting eldest entity
			return e.moveHead(), true
		}
	}
}

func (e entityList) size() uint64 {
	return e.total
}

func (e entityList) getHead() *cachedEntity {
	return &e.entities[e.head.sliceIndex()]
}

func (e entityList) getTail() *cachedEntity {
	return &e.entities[e.tail.sliceIndex()]
}

func (e *entityList) link(prev doubleLinkedListPointer, next uint32) {
	e.entities[prev.sliceIndex()].next.setPointer(next)
	e.entities[next].prev = prev
}

func (e *entityList) moveHead() uint32 {
	oldHead := e.head.sliceIndex()

	// moves head forward
	e.head = e.getHead().next
	// new head should point to an undefined prev
	e.getHead().prev.setUndefined()

	// invalidates old head
	e.entities[oldHead].id = flow.ZeroID
	e.entities[oldHead].next.setUndefined()
	e.entities[oldHead].prev.setUndefined()

	return oldHead
}
