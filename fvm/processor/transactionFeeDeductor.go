package processor

import (
	"errors"
	"fmt"

	"github.com/onflow/flow-go/fvm"
	"github.com/onflow/flow-go/model/flow"
)

type TransactionFeeDeductor struct{}

func NewTransactionFeeDeductor() *TransactionFeeDeductor {
	return &TransactionFeeDeductor{}
}

func (d *TransactionFeeDeductor) Process(
	vm *fvm.VirtualMachine,
	proc fvm.Procedure,
	env fvm.Environment,
) error {
	var txProc fvm.TransactionProcedure
	var ok bool
	if accountProc, ok := proc.(fvm.TransactionProcedure); !ok {
		return errors.New("transaction fee deductor can only process transaction procedures")
	}

	if txProc.Transaction == nil {
		return errors.New("transaction invocator cannot process a procedure with empty transaction")
	}

	return d.deductFees(vm, txProc.Transaction, env)
}

// TODO load the fees from txProc or env
func (d *TransactionFeeDeductor) deductFees(
	vm *VirtualMachine,
	tx *flow.TransactionBody,
	env fvm.Environment,
) error {
	return vm.invokeMetaTransaction(
		deductTransactionFeeTransaction(tx.Payer, ctx.Chain.ServiceAddress()),
		env,
	)
}

const deductTransactionFeeTransactionTemplate = `
import FlowServiceAccount from 0x%s

transaction {
  prepare(account: AuthAccount) {
 	FlowServiceAccount.deductTransactionFee(account)
  }
}
`

func deductTransactionFeeTransaction(accountAddress, serviceAddress flow.Address) *TransactionProcedure {
	return Transaction(
		flow.NewTransactionBody().
			SetScript([]byte(fmt.Sprintf(deductTransactionFeeTransactionTemplate, serviceAddress))).
			AddAuthorizer(accountAddress),
		0,
	)
}
