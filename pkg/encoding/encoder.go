package encoding

import "github.com/dapperlabs/flow-go/pkg/types"

type Encoder interface {
	EncodeTransaction(*types.Transaction) ([]byte, error)
	EncodeCanonicalTransaction(*types.Transaction) ([]byte, error)

	EncodeAccountPublicKey(*types.AccountPublicKey) ([]byte, error)
	DecodeAccountPublicKey([]byte) (*types.AccountPublicKey, error)
}
