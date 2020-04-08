package hash

import (
	"github.com/dapperlabs/flow-go/crypto/hash"
)

// DefaultHasher is the default hasher used by Flow.
var DefaultHasher hash.Hasher

func init() {
	DefaultHasher = hash.NewSHA3_256()
}
