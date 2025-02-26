package stdmap

import (
	"math"
	"sync"

	"github.com/onflow/flow-go/module/mempool"
	"github.com/onflow/flow-go/module/mempool/stdmap/backdata"
)

// Backend is a wrapper around the backdata that provides concurrency-safe operations.
type Backend[K comparable, V any] struct {
	sync.RWMutex
	// The generic key-value storage.
	backData mempool.BackData[K, V]

	guaranteedCapacity uint
	batchEject         BatchEjectFunc[K, V]
	eject              EjectFunc[K, V]
	ejectionCallbacks  []mempool.OnEjection[V]
}

func NewBackend[K comparable, V any](options ...OptionFunc[K, V]) *Backend[K, V] {
	b := Backend[K, V]{
		backData:           backdata.NewMapBackData[K, V](),
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
