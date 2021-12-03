package backdata

import (
	"math/rand"

	"github.com/onflow/flow-go/model/flow"
)

type doubleLinkedListNode struct {
	next uint64
	prev uint64
}

type cachedEntity struct {
	doubleLinkedListNode
	id     flow.Identifier
	owner  uint64
	entity flow.Entity
}

type entityList struct {
	total        uint64
	head         int // index of the head of linked list in entities slice.
	entities     []cachedEntity
	ejectionMode EjectionMode
}

func newEntityList(limit uint64, ejectionMode EjectionMode) *entityList {
	return &entityList{
		head:         -1, // -1 means not-initialized.
		entities:     make([]cachedEntity, limit),
		ejectionMode: ejectionMode,
	}
}

func (e *entityList) add(entityId flow.Identifier, entity flow.Entity, owner uint64) uint32 {
	entityIndex, ejection := e.valueIndexForEntity()
	e.entities[entityIndex].entity = entity
	e.entities[entityIndex].id = entityId
	e.entities[entityIndex].owner = owner

	if !ejection {
		e.total++
	}

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
			return uint32(e.total % limit), true
		}
	}
}

func (e entityList) size() uint64 {
	return e.total
}
