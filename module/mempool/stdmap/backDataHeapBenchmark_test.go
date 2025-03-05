package stdmap_test

import (
	"runtime"
	"runtime/debug"
	"sync"
	"testing"
	"time"

	lru "github.com/hashicorp/golang-lru"
	zlog "github.com/rs/zerolog/log"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	herocache "github.com/onflow/flow-go/module/mempool/herocache/backdata"
	"github.com/onflow/flow-go/module/mempool/herocache/backdata/heropool"
	"github.com/onflow/flow-go/module/mempool/stdmap"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/utils/unittest"
)

// BenchmarkBaselineLRU benchmarks heap allocation performance of
// hashicorp LRU cache with 50K capacity against writing 100M entities,
// with Garbage Collection (GC) disabled.
func BenchmarkBaselineLRU(b *testing.B) {
	unittest.SkipBenchmarkUnless(b, unittest.BENCHMARK_EXPERIMENT, "skips benchmarking baseline LRU, set environment variable to enable")

	defer debug.SetGCPercent(debug.SetGCPercent(-1)) // disable GC

	limit := uint(50)
	backData := stdmap.NewBackend[flow.Identifier, *unittest.MockEntity](
		stdmap.WithMutableBackData[flow.Identifier, *unittest.MockEntity](newBaselineLRU[flow.Identifier, *unittest.MockEntity](int(limit))),
		stdmap.WithLimit[flow.Identifier, *unittest.MockEntity](limit))

	entities := unittest.EntityListFixture(uint(100_000))
	testAddEntities(b, limit, backData, entities)

	unittest.PrintHeapInfo(unittest.Logger()) // heap info after writing 100M entities
	gcAndWriteHeapProfile()                   // runs garbage collection
	unittest.PrintHeapInfo(unittest.Logger()) // heap info after running garbage collection
}

// BenchmarkArrayBackDataLRU benchmarks heap allocation performance of
// ArrayBackData-based cache (aka heroCache) with 50K capacity against writing 100M entities,
// with Garbage Collection (GC) disabled.
func BenchmarkArrayBackDataLRU(b *testing.B) {
	defer debug.SetGCPercent(debug.SetGCPercent(-1)) // disable GC
	limit := uint(50_000)

	backData := stdmap.NewBackend[flow.Identifier, *unittest.MockEntity](
		stdmap.WithMutableBackData[flow.Identifier, *unittest.MockEntity](
			herocache.NewCache(
				uint32(limit),
				8,
				heropool.LRUEjection,
				unittest.Logger(),
				metrics.NewNoopCollector())),
		stdmap.WithLimit[flow.Identifier, *unittest.MockEntity](limit))

	entities := unittest.EntityListFixture(uint(100_000_000))
	testAddEntities(b, limit, backData, entities)

	unittest.PrintHeapInfo(unittest.Logger()) // heap info after writing 100M entities
	gcAndWriteHeapProfile()                   // runs garbage collection
	unittest.PrintHeapInfo(unittest.Logger()) // heap info after running garbage collection
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
func testAddEntities(t testing.TB, limit uint, b *stdmap.Backend[flow.Identifier, *unittest.MockEntity], entities []*unittest.MockEntity) {
	// adding elements
	t1 := time.Now()
	for i, e := range entities {
		require.False(t, b.Has(e.ID()))
		// adding each element must be successful.
		require.True(t, b.Add(e.ID(), e))

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
		actual, ok := b.Get(e.ID())
		require.True(t, ok)
		require.Equal(t, *e, actual)
	}
	elapsed := time.Since(t1)
	zlog.Info().Dur("interaction_time", elapsed).Msg("adding elements done")
}

// baseLineLRU implements a BackData wrapper around hashicorp lru, which makes
// it compliant to be used as BackData component in mempool.Backend. Note that
// this is used only as an experimental baseline, and so it's not exported for
// production.
type baselineLRU[K comparable, V any] struct {
	c     *lru.Cache // used to incorporate an LRU cache
	limit int

	// atomicAdjustMutex is used to synchronize concurrent access to the
	// underlying LRU cache. This is needed because hashicorp LRU does not
	// provide thread-safety for atomic adjust-with-init or get-with-init operations.
	atomicAdjustMutex sync.Mutex
}

func newBaselineLRU[K comparable, V any](limit int) *baselineLRU[K, V] {
	var err error
	c, err := lru.New(limit)
	if err != nil {
		panic(err)
	}

	return &baselineLRU[K, V]{
		c:     c,
		limit: limit,
	}
}

// Has checks if backdata already stores a value under the given key.
func (b *baselineLRU[K, V]) Has(key K) bool {
	_, ok := b.c.Get(key)
	return ok
}

// Add adds the given item to the pool.
func (b *baselineLRU[K, V]) Add(key K, value V) bool {
	b.c.Add(key, value)
	return true
}

// Remove will remove the item with the given hash.
func (b *baselineLRU[K, V]) Remove(key K) (value V, removed bool) {
	value, ok := b.c.Get(key)
	if !ok {
		return value, false
	}

	return value, b.c.Remove(key)
}

// Adjust will adjust the value item using the given function if the given key can be found.
// Returns a bool which indicates whether the value was updated as well as the updated value
func (b *baselineLRU[K, V]) Adjust(key K, f func(V) V) (value V, ok bool) {
	value, removed := b.Remove(key)
	if !removed {
		return value, false
	}

	newValue := f(value)

	b.Add(key, newValue)

	return newValue, true
}

// AdjustWithInit will adjust the value item using the given function if the given key can be found.
// If the key is not found, the init function will be called to create a new value.
// Returns a bool which indicates whether the value was updated as well as the updated value and
// a bool indicating whether the value was initialized.
// Note: this is a benchmark helper, hence, the adjust-with-init provides serializability w.r.t other concurrent adjust-with-init or get-with-init operations,
// and does not provide serializability w.r.t concurrent add, adjust or get operations.
func (b *baselineLRU[K, V]) AdjustWithInit(key K, adjust func(V) V, init func() V) (value V, ok bool) {
	b.atomicAdjustMutex.Lock()
	defer b.atomicAdjustMutex.Unlock()

	if b.Has(key) {
		return b.Adjust(key, adjust)
	}
	added := b.Add(key, init())
	if !added {
		return value, false
	}
	return b.Adjust(key, adjust)
}

// Get returns the given item from the pool.
func (b *baselineLRU[K, V]) Get(key K) (value V, ok bool) {
	value, ok = b.c.Get(key)
	if !ok {
		return value, false
	}

	return value, ok
}

// Size will return the total of the backend.
func (b *baselineLRU[K, V]) Size() uint {
	return uint(b.c.Len())
}

// All returns all entities from the pool.
func (b *baselineLRU[K, V]) All() map[K]V {
	all := make(map[K]V)
	for _, key := range b.c.Keys() {

		entity, ok := b.Get(key)
		if !ok {
			panic("could not retrieve value from mempool")
		}
		all[key] = entity
	}

	return all
}

func (b *baselineLRU[K, V]) Keys() []K {
	keys := make([]K, b.c.Len())
	valueKeys := b.c.Keys()
	total := len(valueKeys)
	for i := 0; i < total; i++ {
		keys[i] = valueKeys[i]
	}
	return keys
}

func (b *baselineLRU[K, V]) Values() []V {
	values := make([]V, b.c.Len())
	valuesIds := b.c.Keys()
	total := len(valuesIds)
	for i := 0; i < total; i++ {
		entity, ok := b.Get(valuesIds[i])
		if !ok {
			panic("could not retrieve entity from mempool")
		}
		values[i] = entity
	}
	return values
}

// Clear removes all entities from the pool.
func (b *baselineLRU[K, V]) Clear() {
	var err error
	b.c, err = lru.New(b.limit)
	if err != nil {
		panic(err)
	}
}
