package heropool

//TODO
// 1. introduce size into linked list struct  and make sure that it is correcly maintained
// by adding checks to existing unit tests. The are several options to do that , bt for now lets maintain size in  basic interf method
// like Add, Remove, init
// 2. start replacing checks for undifined with checks for size , run tests
// 3. finally remove the consept of 0 index as being undefined null ptr index

// TBH I woould decouple lists and poolEntity.
import (
	"fmt"
	"math/rand"

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
			// It's a bit redundant to have a size of free as it can be deduced by
			// len(poolEntities) - used.size. However in task it is specified
			// "Adding a size attribute to track the number of items in the linked list."
			// As there are 2 linked list looks like they suggested to have a size per list.
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

// can be negative
func (p *Pool) modifyUsedBy(incrementBy int) {
	p.used.size += incrementBy
	p.free.size = len(p.poolEntities) - p.used.size
}

// DONE size updated
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
	p.free.size = len(p.poolEntities)
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
		//
		//p.poolEntities[entityIndex].node.next.setUndefined()
		//p.poolEntities[entityIndex].node.prev.setUndefined()

		if p.used.size == 0 {
			// used list is empty, hence setting head of used list to current entityIndex.
			p.used.head.setPoolIndex(entityIndex)
			// as size gonna be non zero tail has to point somewhere and it cant point to 0 anylonger as 0 now
			// is legitim. Lets then make tail and head concide.
			p.used.tail.setPoolIndex(entityIndex)
			// we treat both as undefined prev and next if this node is tail and head so nothing to do
			//p.poolEntities[p.used.head.getSliceIndex()].node.prev.setUndefined()
		} else {
			// if used is non empty then connect to its tail, we expect that eviction conserved valid list
			p.connect(p.used.tail, entityIndex)
			// TODO will it work for corner cases like when capasity of pool is 1 or 2 etc ...
			// since we are appending to the used list, entityIndex also acts as tail of the list.
			p.used.tail.setPoolIndex(entityIndex)
		}
		/*
		   if !p.used.tail.isUndefined() {
		       // links new entity to the tail
		       p.connect(p.used.tail, entityIndex)
		   }*/

		// not sure why here it is incremented as  p.sliceIndexForEntity() couldve evict one element
		// may be check for ejectedEntity ?
		p.size++
		p.modifyUsedBy(1)
	}

	return entityIndex, slotAvailable, ejectedEntity
}

// Get returns entity corresponding to the entity index from the underlying list.
func (p *Pool) Get(entityIndex EIndex) (flow.Identifier, flow.Entity, uint64) {
	return p.poolEntities[entityIndex].id, p.poolEntities[entityIndex].entity, p.poolEntities[entityIndex].owner
}

// All returns all stored entities in this pool.
func (p *Pool) All() []PoolEntity {
	all := make([]PoolEntity, p.size)
	next := p.used.head

	for i := uint32(0); i < p.size; i++ {
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
// TODO update size here
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
func (p *Pool) Size() uint32 {
	return p.size
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
	p.poolEntities[prev.getSliceIndex()].node.next.setPoolIndex(next)
	p.poolEntities[next].node.prev = prev
}

// invalidateUsedHead moves current used head forward by one node. It
// also removes the entity the invalidated head is presenting and appends the
// node represented by the used head to the tail of the free list.
// TODO update size here
func (p *Pool) invalidateUsedHead() flow.Entity {
	headSliceIndex := p.used.head.getSliceIndex()
	return p.invalidateEntityAtIndex(headSliceIndex)
}

// claimFreeHead moves the free head forward, and returns the slice index of the
// old free head to host a new entity.
// TODO update size here

func (p *Pool) claimFreeHead() EIndex {
	oldFreeHeadIndex := p.free.head.getSliceIndex()
	// moves head forward

	if p.free.size == 0 {
		fmt.Println("debug shouldnt happen")
	}

	if p.free.size == 1 {
		p.free.size = 0
	}

	if p.free.size > 1 {
		p.free.head = p.poolEntities[oldFreeHeadIndex].node.next

	}

	// new head should point to an undefined prev,
	// but we first check if list is not empty, i.e.,
	// head itself is not undefined.
	// irrelevant a heads prev is undefined by convention
	//if p.free.size != 0 {
	//  p.poolEntities[p.free.head.getSliceIndex()].node.prev.setUndefined()
	//}

	// also, we check if the old head and tail are aligned and, if so, update the
	// tail as well. This happens when we claim the only existing
	// node of the free list.
	//same not needed as size set to 0
	//if p.free.tail.getSliceIndex() == oldFreeHeadIndex {
	//  p.free.tail.setUndefined()
	//}

	// clears pointers of claimed head
	//p.poolEntities[oldFreeHeadIndex].node.next.setUndefined()
	//p.poolEntities[oldFreeHeadIndex].node.prev.setUndefined()

	return oldFreeHeadIndex
}

// Remove removes entity corresponding to given getSliceIndex from the list.
// TODO update size here
func (p *Pool) Remove(sliceIndex EIndex) flow.Entity {
	return p.invalidateEntityAtIndex(sliceIndex)
}

// invalidateEntityAtIndex invalidates the given getSliceIndex in the linked list by
// removing its corresponding linked-list node from the used linked list, and appending
// it to the tail of the free list. It also removes the entity that the invalidated node is presenting.
// TODO update size here

func (p *Pool) invalidateEntityAtIndex(sliceIndex EIndex) flow.Entity {
	poolEntity := p.poolEntities[sliceIndex]
	prev := poolEntity.node.prev
	next := poolEntity.node.next
	invalidatedEntity := poolEntity.entity

	if p.used.size == 0 {
		fmt.Println("Debug shouldnt happen")
		//this function works only when called on nonempty ued list. would be nice to have
		// panic like a debug assert  later
		return invalidatedEntity

	}
	if p.used.size == 1 {
		// decrements Size
		//se could set here p.ued.head.prev and next to 0s but its not needed
		p.poolEntities[sliceIndex].id = flow.ZeroID
		p.poolEntities[sliceIndex].entity = nil
		p.appendToFreeList(sliceIndex)
		p.size--
		p.modifyUsedBy(-1)

		return invalidatedEntity
	}
	// here size > 1

	// 3 cases index is middle, head or last of used
	if sliceIndex != p.used.head.getSliceIndex() && sliceIndex != p.used.tail.getSliceIndex() {
		// links next and prev elements for non-head and non-tail element
		p.connect(prev, next.getSliceIndex())
	}

	if sliceIndex == p.used.head.getSliceIndex() {
		// invalidating used head
		// moves head forward
		oldUsedHead, _ := p.getHeads()
		p.used.head = oldUsedHead.node.next
		// new head should point to an undefined prev,
		// but we first check if list is not empty, i.e.,
		// head itself is not undefined.
		// if !p.used.head.isUndefined()
		//not needed anymore
		//if !p.used.head.isUndefined() {
		//	usedHead, _ := p.getHeads()
		//	usedHead.node.prev.setUndefined()
		//}
	}

	if sliceIndex == p.used.tail.getSliceIndex() {
		// invalidating used tail
		// moves tail backward
		oldUsedTail, _ := p.getTails()
		p.used.tail = oldUsedTail.node.prev
		// new head should point tail to an undefined next,
		// but we first check if list is not empty, i.e.,
		// tail itself is not undefined.
		//not needed
		//if !p.used.tail.isUndefined() {
		//	usedTail, _ := p.getTails()
		//	usedTail.node.next.setUndefined()
		//}
	}

	// invalidates entity and adds it to free entities.
	p.poolEntities[sliceIndex].id = flow.ZeroID
	p.poolEntities[sliceIndex].entity = nil
	//p.poolEntities[sliceIndex].node.next.setUndefined()
	//p.poolEntities[sliceIndex].node.prev.setUndefined()

	p.appendToFreeList(sliceIndex)

	// decrements Size
	p.size--
	p.modifyUsedBy(-1)

	return invalidatedEntity
}

// appendToFreeList appends linked-list node represented by getSliceIndex to tail of free list.
func (p *Pool) appendToFreeList(sliceIndex EIndex) {
	if p.free.size == 0 {
		// free list is empty
		p.free.head.setPoolIndex(sliceIndex)
		p.free.tail.setPoolIndex(sliceIndex)
		return
	}

	// appends to the tail, and updates the tail
	p.connect(p.free.tail, sliceIndex)
	p.free.tail.setPoolIndex(sliceIndex)
	// it's gonna be reupdated but its a good practice to maintain size in sync
	p.free.size++
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
