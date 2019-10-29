package encoding

import (
	"github.com/dapperlabs/flow-go/pkg/encoding/rlp"
	"github.com/dapperlabs/flow-go/pkg/types"
)

type Encoder interface {
	EncodeTransaction(types.Transaction) ([]byte, error)

	EncodeAccountPublicKey(types.AccountPublicKey) ([]byte, error)
	DecodeAccountPublicKey([]byte) (types.AccountPublicKey, error)

	EncodeAccountPrivateKey(types.AccountPrivateKey) ([]byte, error)
	DecodeAccountPrivateKey([]byte) (types.AccountPrivateKey, error)

	EncodeChunk(types.Chunk) ([]byte, error)

	EncodeCollection(types.Collection) ([]byte, error)
}

var DefaultEncoder = rlp.NewEncoder()
