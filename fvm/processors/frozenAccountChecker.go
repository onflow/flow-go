package processors

import (
	"fmt"

	"github.com/onflow/flow-go/fvm/state"
	"github.com/onflow/flow-go/model/flow"
)

type AccountFrozenChecker struct{}

func (AccountFrozenChecker) Check(
	tx *flow.TransactionBody,
	accounts *state.Accounts,
) error {
	for _, authorizer := range tx.Authorizers {
		err := accounts.CheckAccountNotFrozen(authorizer)
		if err != nil {
			return fmt.Errorf("checking frozen account failed: %w", err)
		}
	}

	err := accounts.CheckAccountNotFrozen(tx.ProposalKey.Address)
	if err != nil {
		return fmt.Errorf("checking frozen account failed: %w", err)
	}

	err = accounts.CheckAccountNotFrozen(tx.Payer)
	if err != nil {
		return fmt.Errorf("checking frozen account failed: %w", err)
	}

	return nil
}
