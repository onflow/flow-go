package fvm

import (
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

	if ctx.ServiceAccountEnabled {
		env := NewScriptEnvironment(ctx, vm, sth, programs)
		balance, err := env.GetAccountBalance(common.BytesToAddress(address.Bytes()))
		if err != nil {
			return nil, err
		}

		account.Balance = balance
	}

	return account, nil
}
