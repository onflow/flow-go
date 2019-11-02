// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package consensus

import (
	"github.com/dapperlabs/flow-go/model/collection"
)

// MempoolResponse is a response for collection mempool contents.
type MempoolResponse struct {
	Nonce       uint64
	Collections []*collection.GuaranteedCollection
}
