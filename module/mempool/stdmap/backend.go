package stdmap

import (
	"math"
	"sync"

	"github.com/onflow/flow-go/module/mempool"
	"github.com/onflow/flow-go/module/mempool/stdmap/backdata"
)

// Backend is a wrapper around the mutable backdata that provides concurrency-safe operations.
type Backend[K comparable, V any] struct {
	sync.RWMutex
	mutableBackData    mempool.MutableBackData[K, V]
	guaranteedCapacity uint
	batchEject         BatchEjectFunc[K, V]
	eject              EjectFunc[K, V]
	ejectionCallbacks  []mempool.OnEjection[V]
}

// NewBackend creates a new memory pool backend.
// This is using EjectRandomFast()
func NewBackend[K comparable, V any](options ...OptionFunc[K, V]) *Backend[K, V] {
	b := Backend[K, V]{
		mutableBackData:    backdata.NewMapBackData[K, V](),
		guaranteedCapacity: uint(math.MaxUint32),
		batchEject:         EjectRandomFast[K, V],
		eject:              nil,
		ejectionCallbacks:  nil,
	}
	for _, option := range options {
		option(&b)
	}
	return &b
}

// Has checks if a value is stored under the given key.
func (b *Backend[K, V]) Has(key K) bool {
	// bs1 := binstat.EnterTime(binstat.BinStdmap + ".r_lock.(Backend)Has")
	b.RLock()
	// binstat.Leave(bs1)

	// bs2 := binstat.EnterTime(binstat.BinStdmap + ".inlock.(Backend)Has")
	// defer binstat.Leave(bs2)
	defer b.RUnlock()
	has := b.mutableBackData.Has(key)
	return has
}

// Add attempts to add the given value, without overwriting existing data.
// If a value is already stored under the input key, Add is a no-op and returns false.
// If no value is stored under the input key, Add adds the value and returns true.
func (b *Backend[K, V]) Add(key K, value V) bool {
	// bs1 := binstat.EnterTime(binstat.BinStdmap + ".w_lock.(Backend)Add")
	b.Lock()
	// binstat.Leave(bs1)

	// bs2 := binstat.EnterTime(binstat.BinStdmap + ".inlock.(Backend)Add")
	// defer binstat.Leave(bs2)
	defer b.Unlock()
	added := b.mutableBackData.Add(key, value)
	b.reduce()
	return added
}

// Remove removes the value with the given key.
// If the key-value pair exists, returns the value and true.
// Otherwise returns the zero value for type V and false.
func (b *Backend[K, V]) Remove(key K) bool {
	// bs1 := binstat.EnterTime(binstat.BinStdmap + ".w_lock.(Backend)Remove")
	b.Lock()
	// binstat.Leave(bs1)

	// bs2 := binstat.EnterTime(binstat.BinStdmap + ".inlock.(Backend)Remove")
	// defer binstat.Leave(bs2)
	defer b.Unlock()
	_, removed := b.mutableBackData.Remove(key)
	return removed
}

// Adjust will adjust the value item using the given function if the given key can be found.
// Returns a bool which indicates whether the value was updated.
func (b *Backend[K, V]) Adjust(key K, f func(V) (K, V)) (V, bool) {
	// bs1 := binstat.EnterTime(binstat.BinStdmap + ".w_lock.(Backend)Adjust")
	b.Lock()
	// binstat.Leave(bs1)

	// bs2 := binstat.EnterTime(binstat.BinStdmap + ".inlock.(Backend)Adjust")
	// defer binstat.Leave(bs2)
	defer b.Unlock()
	value, wasUpdated := b.mutableBackData.Adjust(key, f)
	return value, wasUpdated
}

// AdjustWithInit adjusts the value using the given function if the given identifier can be found. When the
// value is not found, it initializes the value using the given init function and then applies the adjust function.
// Args:
// - key: the identifier of the value to adjust.
// - adjust: the function that adjusts the value.
// - init: the function that initializes the value when it is not found.
// Returns:
//   - the adjusted value.
//
// - a bool which indicates whether the value was adjusted.
func (b *Backend[K, V]) AdjustWithInit(key K, adjust func(V) (K, V), init func() V) (V, bool) {
	b.Lock()
	defer b.Unlock()

	return b.mutableBackData.AdjustWithInit(key, adjust, init)
}

// GetWithInit returns the given value from the backdata. If the value does not exist, it creates a new value
// using the factory function and stores it in the backdata.
// Args:
// - key: the identifier of the value to get.
// - init: the function that initializes the value when it is not found.
// Returns:
//   - the value.
//
// - a bool which indicates whether the value was found (or created).
func (b *Backend[K, V]) GetWithInit(key K, init func() V) (V, bool) {
	b.Lock()
	defer b.Unlock()

	return b.mutableBackData.GetWithInit(key, init)
}

// ByID returns the given item from the pool.
func (b *Backend[K, V]) ByID(key K) (V, bool) {
	// bs1 := binstat.EnterTime(binstat.BinStdmap + ".r_lock.(Backend)ByID")
	b.RLock()
	// binstat.Leave(bs1)

	// bs2 := binstat.EnterTime(binstat.BinStdmap + ".inlock.(Backend)ByID")
	// defer binstat.Leave(bs2)
	defer b.RUnlock()
	value, exists := b.mutableBackData.ByID(key)
	return value, exists
}

// Run executes a function giving it exclusive access to the backdata
func (b *Backend[K, V]) Run(f func(backdata mempool.BackData[K, V]) error) error {
	// bs1 := binstat.EnterTime(binstat.BinStdmap + ".w_lock.(Backend)Run")
	b.Lock()
	// binstat.Leave(bs1)

	// bs2 := binstat.EnterTime(binstat.BinStdmap + ".inlock.(Backend)Run")
	// defer binstat.Leave(bs2)
	defer b.Unlock()
	err := f(b.mutableBackData)
	b.reduce()
	return err
}

// Size will return the size of the backend.
func (b *Backend[K, V]) Size() uint {
	// bs1 := binstat.EnterTime(binstat.BinStdmap + ".r_lock.(Backend)Size")
	b.RLock()
	// binstat.Leave(bs1)

	// bs2 := binstat.EnterTime(binstat.BinStdmap + ".inlock.(Backend)Size")
	// defer binstat.Leave(bs2)
	defer b.RUnlock()
	size := b.mutableBackData.Size()
	return size
}

// Limit returns the maximum number of items allowed in the backend.
func (b *Backend[K, V]) Limit() uint {
	return b.guaranteedCapacity
}

// All returns all entities from the pool.
func (b *Backend[K, V]) All() []V {
	// bs1 := binstat.EnterTime(binstat.BinStdmap + ".r_lock.(Backend)All")
	b.RLock()
	// binstat.Leave(bs1)

	// bs2 := binstat.EnterTime(binstat.BinStdmap + ".inlock.(Backend)All")
	// defer binstat.Leave(bs2)
	defer b.RUnlock()

	return b.mutableBackData.Entities()
}

// Clear removes all entities from the pool.
func (b *Backend[K, V]) Clear() {
	// bs1 := binstat.EnterTime(binstat.BinStdmap + ".w_lock.(Backend)Clear")
	b.Lock()
	// binstat.Leave(bs1)

	// bs2 := binstat.EnterTime(binstat.BinStdmap + ".inlock.(Backend)Clear")
	// defer binstat.Leave(bs2)
	defer b.Unlock()
	b.mutableBackData.Clear()
}

// RegisterEjectionCallbacks adds the provided OnEjection callbacks
func (b *Backend[K, V]) RegisterEjectionCallbacks(callbacks ...mempool.OnEjection[V]) {
	// bs1 := binstat.EnterTime(binstat.BinStdmap + ".r_lock.(Backend)RegisterEjectionCallbacks")
	b.Lock()
	// binstat.Leave(bs1)

	// bs2 := binstat.EnterTime(binstat.BinStdmap + ".inlock.(Backend)RegisterEjectionCallbacks")
	// defer binstat.Leave(bs2)
	defer b.Unlock()
	b.ejectionCallbacks = append(b.ejectionCallbacks, callbacks...)
}

// reduce will reduce the size of the kept entities until we are within the
// configured memory pool size limit.
func (b *Backend[K, V]) reduce() {
	// bs := binstat.EnterTime(binstat.BinStdmap + ".??lock.(Backend)reduce")
	// defer binstat.Leave(bs)

	// we keep reducing the cache size until we are at limit again
	// this was a loop, but the loop is now in EjectRandomFast()
	// the ejections are batched, so this call to eject() may not actually
	// do anything until the batch threshold is reached (currently 128)
	if b.mutableBackData.Size() > b.guaranteedCapacity {
		// get the key from the eject function
		// we don't do anything if there is an error
		if b.batchEject != nil {
			_, _ = b.batchEject(b)
		} else {
			_, _, _ = b.eject(b)
		}
	}
}
