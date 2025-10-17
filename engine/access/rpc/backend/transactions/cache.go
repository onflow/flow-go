package transactions

import (
	accessmodel "github.com/onflow/flow-go/model/access"
	"github.com/onflow/flow-go/model/flow"
)

// TxResultCache is a cache for transaction results used by the API to avoid unnecessary lookups on
// historical access nodes.
type TxResultCache interface {
	Get(flow.Identifier) (*accessmodel.TransactionResult, bool)
	Add(flow.Identifier, *accessmodel.TransactionResult) bool
}

// NoopTxResultCache is a no-op implementation of the TxResultCache interface.
type NoopTxResultCache struct{}

var _ TxResultCache = (*NoopTxResultCache)(nil)

func (n *NoopTxResultCache) Get(flow.Identifier) (*accessmodel.TransactionResult, bool) {
	return nil, false
}

func (n *NoopTxResultCache) Add(flow.Identifier, *accessmodel.TransactionResult) bool {
	return false
}
