package encoding

import (
	rlp2 "github.com/dapperlabs/flow-go/model/encoding/rlp"
	"github.com/dapperlabs/flow-go/model/flow"
)

type Encoder interface {
	EncodeTransaction(flow.Transaction) ([]byte, error)

	EncodeAccountPublicKey(flow.AccountPublicKey) ([]byte, error)
	DecodeAccountPublicKey([]byte) (flow.AccountPublicKey, error)

	EncodeAccountPrivateKey(flow.AccountPrivateKey) ([]byte, error)
	DecodeAccountPrivateKey([]byte) (flow.AccountPrivateKey, error)

	EncodeChunk(flow.Chunk) ([]byte, error)

	EncodeCollection(flow.Collection) ([]byte, error)
}

var DefaultEncoder = rlp2.NewEncoder()
