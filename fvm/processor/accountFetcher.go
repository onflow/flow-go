package processor

// TODO Ramtin update this to be a process

import (
	"errors"
	"fmt"

	"github.com/onflow/flow-go/fvm"
	"github.com/onflow/flow-go/fvm/state"
	"github.com/onflow/flow-go/model/flow"
	"github.com/rs/zerolog"
)

type AccountFetcher struct {
	logger zerolog.Logger
}

func NewAccountFetcher(logger zerolog.Logger) *AccountFetcher {
	return &AccountFetcher{
		logger: logger,
	}
}

func (i *AccountOperator) Process(
	vm fvm.VirtualMachine,
	proc fvm.Procedure,
	env fvm.Environment,
) error {

	var accountProc fvm.AccountProcedure
	var ok bool
	if accountProc, ok := proc.(fvm.AccountProcedure); !ok {
		return errors.New("account fetcher can only process account procedures")
	}

	account, err := getAccount(vm, env, proc.Chain(), proc.Address())

	vmErr, fatalErr := fvm.HandleError(err)
	if fatalErr != nil {
		return fatalErr
	}

	if vmErr != nil {
		proc.Err = vmErr
		return nil
	}

	proc.Account = account
	return nil
}

func getAccount(
	vm *fvm.VirtualMachine,
	env fvm.Environment,
	chain flow.Chain,
	address flow.Address,
) (*flow.Account, error) {
	accounts := env.Accounts()
	account, err := accounts.Get(address)
	if err != nil {
		if errors.Is(err, state.ErrAccountNotFound) {
			return nil, ErrAccountNotFound
		}

		return nil, err
	}

	if env.ctx.ServiceAccountEnabled {
		script := getFlowTokenBalanceScript(address, chain.ServiceAddress())

		err = vm.Run(
			ctx,
			script,
			st.Ledger(),
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

const initFlowTokenTransactionTemplate = `
import FlowServiceAccount from 0x%s

transaction {
  prepare(account: AuthAccount) {
    FlowServiceAccount.initDefaultToken(account)
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

func initFlowTokenTransaction(accountAddress, serviceAddress flow.Address) *TransactionProcedure {
	return Transaction(
		flow.NewTransactionBody().
			SetScript([]byte(fmt.Sprintf(initFlowTokenTransactionTemplate, serviceAddress))).
			AddAuthorizer(accountAddress),
		0,
	)
}

func getFlowTokenBalanceScript(accountAddress, serviceAddress flow.Address) *ScriptProcedure {
	return Script([]byte(fmt.Sprintf(getFlowTokenBalanceScriptTemplate, serviceAddress, accountAddress)))
}
