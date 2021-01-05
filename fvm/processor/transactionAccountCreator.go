package processor

import (
	"fmt"

	"github.com/onflow/cadence/runtime"
	"github.com/onflow/flow-go/fvm"
	"github.com/onflow/flow-go/model/flow"
)

type TransactionAccountCreator struct{}

func NewTransactionAccountCreator() *TransactionAccountCreator {
	return &TransactionAccountCreator{}
}

func (d *TransactionFeeDeductor) Process(
	vm *fvm.VirtualMachine,
	proc fvm.Procedure,
	env fvm.Environment,
) error {
	return d.createAccount(vm, proc, env)
}

func (d *TransactionFeeDeductor) createAccount(
	vm *VirtualMachine,
	tx *flow.TransactionBody,
	env fvm.Environment,
) error {
	if env.ctx.ServiceAccountEnabled {
		err = vm.invokeMetaTransaction(
			deductAccountCreationFeeTransaction(
				flow.Address(payer),
				e.ctx.Chain.ServiceAddress(),
				e.ctx.RestrictedAccountCreationEnabled,
			),
			env,
		)
		if err != nil {
			// TODO: improve error passing https://github.com/onflow/cadence/issues/202
			return address, err
		}
	}

	flowAddress, err := env.AddressGenerator().NextAddress()
	if err != nil {
		return address, err
	}

	err = env.Accounts().Create(nil, flowAddress)
	if err != nil {
		// TODO: improve error passing https://github.com/onflow/cadence/issues/202
		return address, err
	}

	if env.ctx.ServiceAccountEnabled {
		err = vm.invokeMetaTransaction(
			initFlowTokenTransaction(flowAddress, env.ctx.Chain.ServiceAddress()),
			env,
		)
		if err != nil {
			// TODO: improve error passing https://github.com/onflow/cadence/issues/202
			return address, err
		}
	}

	append(proc.NewAccounts, flowAddress)
	return nil
}

const deductAccountCreationFeeTransactionTemplate = `
import FlowServiceAccount from 0x%s

transaction {
  prepare(account: AuthAccount) {
    FlowServiceAccount.deductAccountCreationFee(account)
  }
}
`

const deductAccountCreationFeeWithAllowlistTransactionTemplate = `
import FlowServiceAccount from 0x%s
	
transaction {
  prepare(account: AuthAccount) {
    if !FlowServiceAccount.isAccountCreator(account.address) {
	  panic("Account not authorized to create accounts")
    }

    FlowServiceAccount.deductAccountCreationFee(account)
  }
}
`

func deductAccountCreationFeeTransaction(
	accountAddress, serviceAddress flow.Address,
	restrictedAccountCreationEnabled bool,
) *TransactionProcedure {
	var script string

	if restrictedAccountCreationEnabled {
		script = deductAccountCreationFeeWithAllowlistTransactionTemplate
	} else {
		script = deductAccountCreationFeeTransactionTemplate
	}

	tx := flow.NewTransactionBody().
		SetScript([]byte(fmt.Sprintf(script, serviceAddress))).
		AddAuthorizer(accountAddress)

	return Transaction(tx, 0)
}
