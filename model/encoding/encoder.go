package encoding

import (
	"github.com/dapperlabs/flow-go/model/encoding/rlp"
)

// Encoder encodes and decodes values to and from bytes.
type Encoder interface {
	Encode(interface{}) ([]byte, error)
	Decode(interface{}, []byte) error
}

// DefaultEncoder is the default encoder used by Flow.
var DefaultEncoder = rlp.NewEncoder()
