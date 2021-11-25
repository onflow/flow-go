package stdmap

import (
	crand "crypto/rand"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
)

// TestLRUEjector_Track evaluates that tracking a new item adds the item to the ejector table.
func TestLRUEjector_Track(t *testing.T) {
	ejector := NewLRUEjector()
	// ejector's table should be empty
	assert.Len(t, ejector.table, 0)

	// sequence number of ejector should initially be zero
	assert.Equal(t, ejector.seqNum, uint64(0))

	// creates adds an item to the ejector
	item := flow.Identifier{0x00}
	ejector.Track(item)

	// size of ejector's table should be one
	// which indicates that ejector is tracking the item
	assert.Len(t, ejector.table, 1)

	// item should reside in the ejector's table
	_, ok := ejector.table[item]
	assert.True(t, ok)

	// sequence number of ejector should be increased by one
	assert.Equal(t, ejector.seqNum, uint64(1))
}

// TestLRUEjector_Track_Duplicate evaluates that tracking a duplicate item
// does not change the internal state of the ejector.
func TestLRUEjector_Track_Duplicate(t *testing.T) {
	ejector := NewLRUEjector()

	// creates adds an item to the ejector
	item := flow.Identifier{0x00}
	ejector.Track(item)

	// size of ejector's table should be one
	// which indicates that ejector is tracking the item
	assert.Len(t, ejector.table, 1)

	// item should reside in the ejector's table
	_, ok := ejector.table[item]
	assert.True(t, ok)

	// sequence number of ejector should be increased by one
	assert.Equal(t, ejector.seqNum, uint64(1))

	// adds the duplicate item
	ejector.Track(item)

	// internal state of the ejector should be unchaged
	assert.Len(t, ejector.table, 1)
	assert.Equal(t, ejector.seqNum, uint64(1))
	_, ok = ejector.table[item]
	assert.True(t, ok)
}

// TestLRUEjector_Track_Many evaluates that tracking many items
// changes the state of ejector properly, i.e., items reside on the
// memory, and sequence number changed accordingly.
func TestLRUEjector_Track_Many(t *testing.T) {
	ejector := NewLRUEjector()

	// creates and tracks 100 items
	size := 100
	items := flow.IdentifierList{}
	for i := 0; i < size; i++ {
		var id flow.Identifier
		_, _ = crand.Read(id[:])
		ejector.Track(id)
		items = append(items, id)
	}

	// size of ejector's table should be 100
	assert.Len(t, ejector.table, size)

	// all items should reside in the ejector's table
	for _, id := range items {
		_, ok := ejector.table[id]
		require.True(t, ok)
	}

	// sequence number of ejector should be increased by size
	assert.Equal(t, ejector.seqNum, uint64(size))
}

// TestLRUEjector_Untrack_One evaluates that untracking an existing item
// removes it from the ejector state and changes the state accordingly.
func TestLRUEjector_Untrack_One(t *testing.T) {
	ejector := NewLRUEjector()

	// creates adds an item to the ejector
	item := flow.Identifier{0x00}
	ejector.Track(item)

	// size of ejector's table should be one
	// which indicates that ejector is tracking the item
	assert.Len(t, ejector.table, 1)

	// item should reside in the ejector's table
	_, ok := ejector.table[item]
	assert.True(t, ok)

	// sequence number of ejector should be increased by one
	assert.Equal(t, ejector.seqNum, uint64(1))

	// untracks the item
	ejector.Untrack(item)

	// internal state of the ejector should be changed
	assert.Len(t, ejector.table, 0)

	// sequence number should not be changed
	assert.Equal(t, ejector.seqNum, uint64(1))

	// item should no longer reside on internal state of ejector
	_, ok = ejector.table[item]
	assert.False(t, ok)
}

// TestLRUEjector_Untrack_Duplicate evaluates that untracking an item twice
// removes it from the ejector state only once and changes the state safely.
func TestLRUEjector_Untrack_Duplicate(t *testing.T) {
	ejector := NewLRUEjector()

	// creates and adds two items to the ejector
	item1 := flow.Identifier{0x00}
	item2 := flow.Identifier{0x01}
	ejector.Track(item1)
	ejector.Track(item2)

	// size of ejector's table should be two
	// which indicates that ejector is tracking the items
	assert.Len(t, ejector.table, 2)

	// items should reside in the ejector's table
	_, ok := ejector.table[item1]
	assert.True(t, ok)
	_, ok = ejector.table[item2]
	assert.True(t, ok)

	// sequence number of ejector should be increased by two
	assert.Equal(t, ejector.seqNum, uint64(2))

	// untracks the item twice
	ejector.Untrack(item1)
	ejector.Untrack(item1)

	// internal state of the ejector should be changed
	assert.Len(t, ejector.table, 1)

	// sequence number should not be changed
	assert.Equal(t, ejector.seqNum, uint64(2))

	// double untracking should only affect the untracked item1
	_, ok = ejector.table[item1]
	assert.False(t, ok)

	// item 2 should still reside in the memory
	_, ok = ejector.table[item2]
	assert.True(t, ok)
}

// TestLRUEjector_UntrackEject evaluates that untracking the next ejectable item
// properly changes the next ejectable item in the ejector.
func TestLRUEjector_UntrackEject(t *testing.T) {
	ejector := NewLRUEjector()

	// creates and tracks 100 items
	size := 100
	var bkend Backend

	items := make([]flow.Identifier, size)
	entities := make(map[flow.Identifier]flow.Entity)

	for i := 0; i < size; i++ {
		var id flow.Identifier
		_, _ = crand.Read(id[:])
		ejector.Track(id)

		entities[id] = MockEntity{}
		items[i] = id
	}

	// untracks the oldest item
	ejector.Untrack(items[0])

	bkend.backData = &MapBackData{entities: entities}

	// next ejectable item should be the second oldest item
	id, _, _ := ejector.Eject(&bkend)
	assert.Equal(t, id, items[1])
}

// TestLRUEjector_EjectAll adds many item to the ejector and then ejects them
// all one by one and evaluates an LRU ejection behavior.
func TestLRUEjector_EjectAll(t *testing.T) {
	ejector := NewLRUEjector()

	// creates and tracks 100 items
	size := 100
	var bkend Backend

	items := make([]flow.Identifier, size)
	entities := make(map[flow.Identifier]flow.Entity)
	for i := 0; i < size; i++ {
		var id flow.Identifier
		_, _ = crand.Read(id[:])
		ejector.Track(id)

		entities[id] = MockEntity{}
		items[i] = id
	}

	bkend.backData = &MapBackData{entities: entities}

	// ejects one by one
	for i := 0; i < size; i++ {
		id, _, _ := ejector.Eject(&bkend)
		require.Equal(t, id, items[i])
	}
}

// MockEntity is a mocked entity type used in internal testing of ejectors.
type MockEntity struct{}

func (m MockEntity) ID() flow.Identifier {
	return flow.Identifier{}
}

func (m MockEntity) Checksum() flow.Identifier {
	return flow.Identifier{}
}
