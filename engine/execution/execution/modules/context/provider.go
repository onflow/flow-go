package context

import (
	"github.com/dapperlabs/flow-go/engine/execution/execution/modules/ledger"
	"github.com/dapperlabs/flow-go/language/runtime"
	"github.com/dapperlabs/flow-go/model/flow"
)

// NewProvider returns a new context provider.
func NewProvider() Provider {
	return &provider{}
}

type provider struct{}

func (p *provider) NewTransactionContext(tx *flow.Transaction, ledger *ledger.View) TransactionContext {
	signingAccounts := make([]runtime.Address, len(tx.ScriptAccounts))
	for i, addr := range tx.ScriptAccounts {
		signingAccounts[i] = runtime.Address(addr)
	}

	return &transactionContext{
		ledger:          ledger,
		signingAccounts: signingAccounts,
	}
}
