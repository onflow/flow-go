package fvm

import (
	"github.com/onflow/cadence/runtime/common"

	"github.com/onflow/flow-go/fvm/handler"
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

	computationMeteringHandler := handler.NewComputationMeteringHandler(DefaultGasLimit, handler.WithCoumputationWeightFactors(map[uint]uint{handler.MeteredOperationfunction_or_loop_call: 1}))

	if ctx.ServiceAccountEnabled {
		env := NewScriptEnvironment(ctx, vm, sth, programs, computationMeteringHandler)
		balance, err := env.GetAccountBalance(common.Address(address))
		if err != nil {
			return nil, err
		}

		account.Balance = balance
	}

	return account, nil
}
