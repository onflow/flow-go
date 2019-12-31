package context

import (
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/sdk/abi/values"
)

// NewProvider returns a new context provider.
func NewProvider() Provider {
	return &provider{}
}

type provider struct{}

func (p *provider) NewTransactionContext(tx *flow.Transaction, ledger *flow.LedgerView) TransactionContext {
	signingAccounts := make([]values.Address, len(tx.ScriptAccounts))
	for i, addr := range tx.ScriptAccounts {
		signingAccounts[i] = values.Address(addr)
	}

	return &transactionContext{
		ledger:          ledger,
		signingAccounts: signingAccounts,
	}
}
