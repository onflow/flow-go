package fvm

import (
	"context"

	"github.com/onflow/cadence/runtime/common"

	"github.com/onflow/flow-go/fvm/programs"
	"github.com/onflow/flow-go/fvm/state"
	"github.com/onflow/flow-go/model/flow"
)

func getAccount(
	vm *VirtualMachine,
	ctx Context,
	sth *state.StateHolder,
	programs *programs.Programs,
	address flow.Address,
) (*flow.Account, error) {
	accounts := state.NewAccounts(sth)

	account, err := accounts.Get(address)
	if err != nil {
		return nil, err
	}

	env, err := NewScriptEnvironment(context.Background(), ctx, vm, sth, programs)
	if err != nil {
		return nil, err
	}

	balance, err := env.GetAccountBalance(common.Address(address))
	if err != nil {
		return nil, err
	}

	account.Balance = balance

	return account, nil
}
