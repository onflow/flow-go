package fvm

import (
	"errors"
	"fmt"

	"github.com/onflow/cadence"
	jsoncdc "github.com/onflow/cadence/encoding/json"
	"github.com/onflow/cadence/runtime/common"

	"github.com/onflow/flow-go/fvm/programs"
	"github.com/onflow/flow-go/fvm/state"
	"github.com/onflow/flow-go/model/flow"
)

func getAccount(
	vm *VirtualMachine,
	ctx Context,
	stm *state.StateManager,
	programs *programs.Programs,
	address flow.Address,
) (*flow.Account, error) {
	accounts := state.NewAccounts(stm)

	account, err := accounts.Get(address)
	if err != nil {
		if errors.Is(err, state.ErrAccountNotFound) {
			return nil, ErrAccountNotFound
		}

		return nil, err
	}

	if ctx.ServiceAccountEnabled {
		env, err := newEnvironment(ctx, vm, stm, programs)
		if err != nil {
			return nil, err
		}
		balance, err := env.GetAccountBalance(common.BytesToAddress(address.Bytes()))
		if err != nil {
			return nil, err
		}

		account.Balance = balance
	}

	return account, nil
}

const initAccountTransactionTemplate = `
import FlowServiceAccount from 0x%s

transaction(restrictedAccountCreationEnabled: Bool) {
  prepare(newAccount: AuthAccount, payerAccount: AuthAccount) {
    if restrictedAccountCreationEnabled && !FlowServiceAccount.isAccountCreator(payerAccount.address) {
	  panic("Account not authorized to create accounts")
    }

    FlowServiceAccount.setupNewAccount(newAccount: newAccount, payer: payerAccount)
  }
}
`

const getFlowTokenBalanceScriptTemplate = `
import FlowServiceAccount from 0x%s

pub fun main(): UFix64 {
  let acct = getAccount(0x%s)
  return FlowServiceAccount.defaultTokenBalance(acct)
}
`

const getStorageCapacityScriptTemplate = `
import FlowStorageFees from 0x%s

pub fun main(): UFix64 {
	return FlowStorageFees.calculateAccountCapacity(0x%s)
}
`

func initAccountTransaction(
	payerAddress flow.Address,
	accountAddress flow.Address,
	serviceAddress flow.Address,
	restrictedAccountCreationEnabled bool,
) *TransactionProcedure {
	arg, err := jsoncdc.Encode(cadence.NewBool(restrictedAccountCreationEnabled))
	if err != nil {
		// this should not fail! It simply encodes a boolean
		panic(fmt.Errorf("cannot json encode cadence boolean argument: %w", err))
	}

	return Transaction(
		flow.NewTransactionBody().
			SetScript([]byte(fmt.Sprintf(initAccountTransactionTemplate, serviceAddress))).
			AddAuthorizer(accountAddress).
			AddAuthorizer(payerAddress).
			AddArgument(arg),
		0,
	)
}

func getFlowTokenBalanceScript(accountAddress, serviceAddress flow.Address) *ScriptProcedure {
	return Script([]byte(fmt.Sprintf(getFlowTokenBalanceScriptTemplate, serviceAddress, accountAddress)))
}

func getStorageCapacityScript(accountAddress, serviceAddress flow.Address) *ScriptProcedure {
	return Script([]byte(fmt.Sprintf(getStorageCapacityScriptTemplate, serviceAddress, accountAddress)))
}
