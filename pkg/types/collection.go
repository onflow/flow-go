package types

import (
	"github.com/dapperlabs/flow-go/pkg/crypto"
)

type Collection struct {
	Hash         crypto.Hash
	Transactions []*Transaction
}
