package hash

import (
	"github.com/dapperlabs/flow-go/crypto"
)

// DefaultHasher is the default hasher used by Flow.
var DefaultHasher crypto.Hasher

func init() {
	DefaultHasher = crypto.NewSHA3_256()
}
