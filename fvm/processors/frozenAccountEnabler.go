package processors

import (
	"github.com/onflow/flow-go/fvm/context"
	"github.com/onflow/flow-go/model/flow"
)

type AccountFrozenEnabler struct{}

// TODO ramtin does this really work?, since ctx is called by value???
func (AccountFrozenEnabler) Enable(
	tx *flow.TransactionBody,
	ctx context.Context,
) error {
	serviceAddress := ctx.Chain.ServiceAddress()
	for _, signature := range tx.EnvelopeSignatures {
		if signature.Address == serviceAddress {
			ctx.AccountFreezeAvailable = true
			return nil // we can bail out and save maybe some loops
		}
	}

	return nil
}
