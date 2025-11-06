package access

import (
	"github.com/onflow/flow-go/model/flow"
)

type EventProvider func() (flow.EventsList, error)

// SystemCollectionBuilder defines the builders for the system collections and their transactions.
type SystemCollectionBuilder interface {
	ProcessCallbacksTransaction(chain flow.Chain) (*flow.TransactionBody, error)
	ExecuteCallbacksTransactions(chain flow.Chain, processEvents flow.EventsList) ([]*flow.TransactionBody, error)
	SystemChunkTransaction(chain flow.Chain) (*flow.TransactionBody, error)
	SystemCollection(chain flow.Chain, providerFn EventProvider) (*flow.Collection, error)
}
