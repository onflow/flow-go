// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package flow

import (
	"github.com/dapperlabs/flow-go/crypto"
)

type Collection struct {
	Hash         crypto.Hash
	Transactions []*Transaction
}
