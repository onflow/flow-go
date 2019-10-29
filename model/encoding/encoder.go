package encoding

import (
	"github.com/dapperlabs/flow-go/model/encoding/rlp"
	"github.com/dapperlabs/flow-go/model/flow"
)

// Encoder encodes and decodes Flow models.
type Encoder interface {
	// EncodeTransaction encodes a transaction to bytes.
	EncodeTransaction(flow.Transaction) ([]byte, error)

	// EncodeAccountPublicKey encodes an account public key to bytes.
	EncodeAccountPublicKey(flow.AccountPublicKey) ([]byte, error)
	// DecodeAccountPublicKey decodes an account public key from bytes.
	DecodeAccountPublicKey([]byte) (flow.AccountPublicKey, error)

	// EncodeAccountPrivateKey encodes an account private key to bytes.
	EncodeAccountPrivateKey(flow.AccountPrivateKey) ([]byte, error)
	// DecodeAccountPrivateKey decodes an account private key from bytes.
	DecodeAccountPrivateKey([]byte) (flow.AccountPrivateKey, error)

	// EncodeChunk encodes a chunk to bytes.
	EncodeChunk(flow.Chunk) ([]byte, error)

	// EncodeCollection encodes a collection to bytes.
	EncodeCollection(flow.Collection) ([]byte, error)
}

// DefaultEncoder is the default model encoder.
var DefaultEncoder = rlp.NewEncoder()
