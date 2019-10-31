package hash

import (
	"github.com/dapperlabs/flow-go/crypto"
)

// DefaultHasher is the default hasher used by Flow.
var DefaultHasher crypto.Hasher

func init() {
	var err error
	DefaultHasher, err = crypto.NewHasher(crypto.SHA3_256)
	if err != nil {
		panic(err)
	}
}
