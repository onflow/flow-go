package context

import (
	"github.com/dapperlabs/flow-go/engine/execution/execution/modules/ledger"
	"github.com/dapperlabs/flow-go/language/runtime"
	"github.com/dapperlabs/flow-go/model/flow"
)

// A Provider generates execution contexts to be used for transaction execution.
type Provider interface {
	NewTransactionContext(tx flow.TransactionBody, ledger *ledger.View) TransactionContext
}

type TransactionContext interface {
	runtime.Interface
}
