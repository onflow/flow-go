package fvm

import (
	"fmt"

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
		env := newEnvironment(ctx, vm, sth, programs)
		balance, err := env.GetAccountBalance(common.BytesToAddress(address.Bytes()))
		if err != nil {
			return nil, err
		}

		account.Balance = balance
	}

	return account, nil
}

const getFlowTokenBalanceScriptTemplate = `
import FlowServiceAccount from 0x%s

pub fun main(): UFix64 {
  let acct = getAccount(0x%s)
  return FlowServiceAccount.defaultTokenBalance(acct)
}
`

const getFlowTokenAvailableBalanceScriptTemplate = `
import FlowStorageFees from 0x%s

pub fun main(): UFix64 {
  return FlowStorageFees.defaultTokenAvailableBalance(0x%s)
}
`

const getStorageCapacityScriptTemplate = `
import FlowStorageFees from 0x%s

pub fun main(): UFix64 {
	return FlowStorageFees.calculateAccountCapacity(0x%s)
}
`

func getFlowTokenBalanceScript(accountAddress, serviceAddress flow.Address) *ScriptProcedure {
	return Script([]byte(fmt.Sprintf(getFlowTokenBalanceScriptTemplate, serviceAddress, accountAddress)))
}

func getFlowTokenAvailableBalanceScript(accountAddress, serviceAddress flow.Address) *ScriptProcedure {
	return Script([]byte(fmt.Sprintf(getFlowTokenAvailableBalanceScriptTemplate, serviceAddress, accountAddress)))
}

func getStorageCapacityScript(accountAddress, serviceAddress flow.Address) *ScriptProcedure {
	return Script([]byte(fmt.Sprintf(getStorageCapacityScriptTemplate, serviceAddress, accountAddress)))
}
