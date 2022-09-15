package environment

import (
	"github.com/onflow/cadence/runtime"

	"github.com/onflow/flow-go/fvm/errors"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/trace"
)

// TransactionInfo exposes information associated with the executing
// transaction.
//
// Note that scripts have no associated transaction information, but must expose
// the API in compliance with the runtime environment interface.
type TransactionInfo interface {
	TxIndex() uint32
	TxID() flow.Identifier

	SigningAccounts() []runtime.Address

	IsServiceAccountAuthorizer() bool

	// Cadence's runtime API.  Note that the script variant will return
	// OperationNotSupportedError.
	GetSigningAccounts() ([]runtime.Address, error)
}

type transactionInfo struct {
	txIndex uint32
	txId    flow.Identifier

	tracer *Tracer

	authorizers                []runtime.Address
	isServiceAccountAuthorizer bool
}

func NewTransactionInfo(
	txIndex uint32,
	txId flow.Identifier,
	tracer *Tracer,
	authorizers []flow.Address,
	serviceAccount flow.Address,
) TransactionInfo {

	isServiceAccountAuthorizer := false
	runtimeAddresses := make([]runtime.Address, 0, len(authorizers))

	for _, auth := range authorizers {
		runtimeAddresses = append(runtimeAddresses, runtime.Address(auth))
		if auth == serviceAccount {
			isServiceAccountAuthorizer = true
		}
	}

	return &transactionInfo{
		txIndex:                    txIndex,
		txId:                       txId,
		tracer:                     tracer,
		authorizers:                runtimeAddresses,
		isServiceAccountAuthorizer: isServiceAccountAuthorizer,
	}
}

func (info *transactionInfo) TxIndex() uint32 {
	return info.txIndex
}

func (info *transactionInfo) TxID() flow.Identifier {
	return info.txId
}

func (info *transactionInfo) SigningAccounts() []runtime.Address {
	return info.authorizers
}

func (info *transactionInfo) IsServiceAccountAuthorizer() bool {
	return info.isServiceAccountAuthorizer
}

func (info *transactionInfo) GetSigningAccounts() ([]runtime.Address, error) {
	defer info.tracer.StartExtensiveTracingSpanFromRoot(trace.FVMEnvGetSigningAccounts).End()

	return info.authorizers, nil
}

var _ TransactionInfo = NoTransactionInfo{}

// Scripts have no associated transaction information.
type NoTransactionInfo struct {
}

func (NoTransactionInfo) TxIndex() uint32 {
	return 0
}

func (NoTransactionInfo) TxID() flow.Identifier {
	return flow.ZeroID
}

func (NoTransactionInfo) SigningAccounts() []runtime.Address {
	return nil
}

func (NoTransactionInfo) IsServiceAccountAuthorizer() bool {
	return false
}

func (NoTransactionInfo) GetSigningAccounts() ([]runtime.Address, error) {
	return nil, errors.NewOperationNotSupportedError("GetSigningAccounts")
}
