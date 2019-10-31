package encoding

import (
	"github.com/dapperlabs/flow-go/model/encoding/rlp"
	"github.com/dapperlabs/flow-go/model/flow"
)

// Encoder encodes and decodes values to and from bytes.
type Encoder interface {
	Encode(interface{}) ([]byte, error)
	Decode(interface{}, []byte) error

	// EncodeAccountPublicKey encodes an account public key to bytes.
	EncodeAccountPublicKey(flow.AccountPublicKey) ([]byte, error)
	// DecodeAccountPublicKey decodes an account public key from bytes.
	DecodeAccountPublicKey([]byte) (flow.AccountPublicKey, error)

	// EncodeAccountPrivateKey encodes an account private key to bytes.
	EncodeAccountPrivateKey(flow.AccountPrivateKey) ([]byte, error)
	// DecodeAccountPrivateKey decodes an account private key from bytes.
	DecodeAccountPrivateKey([]byte) (flow.AccountPrivateKey, error)
}

// DefaultEncoder is the default model encoder used by Flow.
var DefaultEncoder = rlp.NewEncoder()
