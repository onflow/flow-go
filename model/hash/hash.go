package hash

import (
	"sync"

	"github.com/dapperlabs/flow-go/crypto/hash"
)

// DefaultHasher is the default hasher used by Flow.
var DefaultHasher hash.Hasher

type defaultHasher struct {
	hash.Hasher
	sync.Mutex
}

func (h *defaultHasher) ComputeHash(b []byte) hash.Hash {
	h.Lock()
	defer h.Unlock()
	return h.Hasher.ComputeHash(b)
}

func init() {
	DefaultHasher = &defaultHasher{
		hash.NewSHA3_256(),
		sync.Mutex{},
	}
}
