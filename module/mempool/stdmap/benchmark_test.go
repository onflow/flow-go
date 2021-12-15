package stdmap_test

import (
	"os"
	"runtime"
	"runtime/debug"
	"strings"
	"testing"
	"time"

	lru "github.com/hashicorp/golang-lru"
	zlog "github.com/rs/zerolog/log"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/mempool/stdmap"
	"github.com/onflow/flow-go/module/mempool/stdmap/backdata"
	"github.com/onflow/flow-go/module/mempool/stdmap/backdata/arraylinkedlist"
	"github.com/onflow/flow-go/utils/unittest"
)

// BenchmarkBaselineLRU benchmarks heap allocation performance of
// hashicorp LRU cache with 50K capacity against writing 100M entities,
// with Garbage Collection (GC) disabled.
func BenchmarkBaselineLRU(b *testing.B) {
	if !experiment() {
		b.Skip("skips benchmarking baseline LRU, set environment variable to enable")
	}
	defer debug.SetGCPercent(debug.SetGCPercent(-1)) // disable GC

	limit := uint(50_000)
	backData := stdmap.NewBackendWithBackData(
		newBaselineLRU(int(limit)),
		stdmap.WithLimit(limit))

	entities := unittest.EntityListFixture(uint(100_000_000))
	testAddEntities(b, limit, backData, entities)

	printHeapInfo()         // heap info after writing 100M entities
	gcAndWriteHeapProfile() // runs garbage collection
	printHeapInfo()         // heap info after running garbage collection
}

// BenchmarkArrayBackDataLRU benchmarks heap allocation performance of
// ArrayBackData-based cache (aka heroCache) with 50K capacity against writing 100M entities,
// with Garbage Collection (GC) disabled.
func BenchmarkArrayBackDataLRU(b *testing.B) {
	defer debug.SetGCPercent(debug.SetGCPercent(-1)) // disable GC
	limit := uint(50_000)

	backData := stdmap.NewBackendWithBackData(
		backdata.NewArrayBackData(uint32(limit),
			8, arraylinkedlist.LRUEjection),
		stdmap.WithLimit(limit))

	entities := unittest.EntityListFixture(uint(100_000_000))
	testAddEntities(b, limit, backData, entities)

	printHeapInfo()         // heap info after writing 100M entities
	gcAndWriteHeapProfile() // runs garbage collection
	printHeapInfo()         // heap info after running garbage collection
}

func printHeapInfo() {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	zlog.Info().
		Uint64(".Alloc", m.Alloc).
		Uint64(".AllocObjects", m.HeapObjects).
		Uint64(".TotalAlloc", m.TotalAlloc).
		Uint32(".NumGC", m.NumGC).
		Msg("printHeapInfo()")
}

func gcAndWriteHeapProfile() {
	// see <https://pkg.go.dev/runtime/pprof>
	t1 := time.Now()
	runtime.GC() // get up-to-date statistics
	elapsed := time.Since(t1).Seconds()
	zlog.Info().
		Float64("gc-elapsed-time", elapsed).
		Msg("garbage collection done")
}

// testAddEntities is a test helper that checks entities are added successfully to the backdata.
// and each entity is retrievable right after it is written to backdata.
func testAddEntities(t testing.TB, limit uint, b *stdmap.Backend, entities []*unittest.MockEntity) {
	// adding elements
	t1 := time.Now()
	for i, e := range entities {
		require.False(t, b.Has(e.ID()))
		// adding each element must be successful.
		require.True(t, b.Add(*e))

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
		require.Equal(t, *e, actual)
	}
	elapsed := time.Since(t1)
	zlog.Info().Dur("interaction_time", elapsed).Msg("adding elements done")
}

// baseLineLRU implements a BackData wrapper around hashicorp lru, which makes
// it compliant to be used as BackData component in mempool.Backend. Note that
// this is used only as an experimental baseline, and hence is not exported for
// production.
type baselineLRU struct {
	c     *lru.Cache // used to incorporate an LRU cache
	limit int
}

func newBaselineLRU(limit int) *baselineLRU {
	var err error
	c, err := lru.New(limit)
	if err != nil {
		panic(err)
	}

	return &baselineLRU{
		c:     c,
		limit: limit,
	}
}

// Has checks if we already contain the item with the given hash.
func (b *baselineLRU) Has(entityID flow.Identifier) bool {
	_, ok := b.c.Get(entityID)
	return ok
}

// Add adds the given item to the pool.
func (b *baselineLRU) Add(entityID flow.Identifier, entity flow.Entity) bool {
	b.c.Add(entityID, entity)
	return true
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

// experiment looks up environment variable to enable/disable benchmarking experiments.
func experiment() bool {
	val, found := os.LookupEnv("experiment")
	if !found {
		return false
	}
	return strings.ToLower(strings.Trim(val, "")) == "on"
}
