package stdmap_test

import (
	"os"
	"runtime"
	"runtime/pprof"
	"testing"
	"time"

	lru "github.com/hashicorp/golang-lru"
	zlog "github.com/rs/zerolog/log"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/mempool/stdmap"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestLRU(t *testing.T) {
	limit := uint(10_000_000)
	b := stdmap.NewBackend(stdmap.WithLimit(limit))

	entities := unittest.EntityListFixture(limit)
	testAddEntities(t, limit, b, entities)

	printHeapInfo()
	gcAndWriteHeapProfile()
	printHeapInfo()
}

func printHeapInfo() {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	zlog.Info().
		Uint64(".Alloc", m.Alloc).
		Uint64(".TotalAlloc", m.TotalAlloc).
		Uint32(".NumGC", m.NumGC).
		Msg("printHeapInfo()")
}

// testAddEntities is a test helper that checks entities are added successfully to the backdata.
// and each entity is retrievable right after it is written to backdata.
func testAddEntities(t *testing.T, limit uint, b *stdmap.Backend, entities []*unittest.MockEntity) {
	// adding elements
	for i, e := range entities {
		// adding each element must be successful.
		require.True(t, b.Add(e))

		if uint(i) < limit {
			// when we are below limit the total of
			// backdata should be incremented by each addition.
			require.Equal(t, b.Size(), uint(i+1))
		} else {
			// when we cross the limit, the ejection kicks in, and
			// size must be steady at the limit.
			require.Equal(t, uint(b.Size()), limit)
		}

		// entity should be immediately retrievable
		actual, ok := b.ByID(e.ID())
		require.True(t, ok)
		require.Equal(t, e, actual)
	}
}

func gcAndWriteHeapProfile() {
	// see <https://pkg.go.dev/runtime/pprof>
	t1 := time.Now()
	runtime.GC() // get up-to-date statistics
	elapsed := time.Since(t1).Seconds()
	zlog.Info().Float64("elapsed", elapsed).Msg("runtime.GC()")
	time.Sleep(2 * time.Second) // presumably GC takes less than 2 seconds?
	zlog.Info().Msg("writing heap profile")
	if err := pprof.WriteHeapProfile(os.Stdout); err != nil {
		panic(err)
	}
}

type baselineLRU struct {
	c     *lru.Cache // used to incorporate an LRU cache
	limit int
}

// Has checks if we already contain the item with the given hash.
func (b *baselineLRU) Has(entityID flow.Identifier) bool {
	_, ok := b.c.Get(entityID)
	return ok
}

// Add adds the given item to the pool.
func (b *baselineLRU) Add(entityID flow.Identifier, entity flow.Entity) bool {
	return b.c.Add(entityID, entity)
}

// Rem will remove the item with the given hash.
func (b *baselineLRU) Rem(entityID flow.Identifier) (flow.Entity, bool) {
	e, ok := b.c.Get(entityID)
	if !ok {
		return nil, false
	}
	entity, ok := e.(flow.Entity)
	if !ok {
		return nil, false
	}

	return entity, b.c.Remove(entityID)
}

// Adjust will adjust the value item using the given function if the given key can be found.
// Returns a bool which indicates whether the value was updated as well as the updated value
func (b *baselineLRU) Adjust(entityID flow.Identifier, f func(flow.Entity) flow.Entity) (flow.Entity, bool) {
	entity, removed := b.Rem(entityID)
	if !removed {
		return nil, false
	}

	newEntity := f(entity)
	newEntityID := newEntity.ID()

	b.Add(newEntityID, newEntity)

	return newEntity, true
}

// ByID returns the given item from the pool.
func (b *baselineLRU) ByID(entityID flow.Identifier) (flow.Entity, bool) {
	e, ok := b.c.Get(entityID)
	if !ok {
		return nil, false
	}

	entity, ok := e.(flow.Entity)
	if !ok {
		return nil, false
	}
	return entity, ok
}

// Size will return the total of the backend.
func (b baselineLRU) Size() uint {
	return uint(b.c.Len())
}

// All returns all entities from the pool.
func (b baselineLRU) All() map[flow.Identifier]flow.Entity {
	all := make(map[flow.Identifier]flow.Entity)
	for _, entityID := range b.c.Keys() {
		id, ok := entityID.(flow.Identifier)
		if !ok {
			panic("could not assert to entity id")
		}

		entity, ok := b.ByID(id)
		if !ok {
			panic("could not retrive entity from mempool")
		}
		all[id] = entity
	}

	return all
}

// Clear removes all entities from the pool.
func (b *baselineLRU) Clear() {
	var err error
	b.c, err = lru.New(b.limit)
	if err != nil {
		panic(err)
	}
}

// Hash will use a merkle root hash to hash all items.
func (b *baselineLRU) Hash() flow.Identifier {
	return flow.MerkleRoot(flow.GetIDs(b.All())...)
}
