package fvm

import (
	"errors"
	"fmt"

	"github.com/onflow/cadence"
	jsoncdc "github.com/onflow/cadence/encoding/json"

	"github.com/onflow/flow-go/fvm/state"
	"github.com/onflow/flow-go/model/flow"
)

func getAccount(
	vm *VirtualMachine,
	ctx Context,
	ledger state.Ledger,
	chain flow.Chain,
	address flow.Address,
) (*flow.Account, error) {
	accounts := state.NewAccounts(ledger)

	account, err := accounts.Get(address)
	if err != nil {
		if errors.Is(err, state.ErrAccountNotFound) {
			return nil, ErrAccountNotFound
		}

		return nil, err
	}

	if ctx.ServiceAccountEnabled {
		script := getFlowTokenBalanceScript(address, chain.ServiceAddress())

		err = vm.Run(
			ctx,
			script,
			ledger,
		)
		if err != nil {
			return nil, err
		}

		var balance uint64

		// TODO: Figure out how to handle this error. Currently if a runtime error occurs, balance will be 0.
		// 1. An error will occur if user has removed their FlowToken.Vault -- should this be allowed?
		// 2. Any other error indicates a bug in our implementation. How can we reliably check the Cadence error?
		if script.Err == nil {
			balance = script.Value.ToGoValue().(uint64)
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
