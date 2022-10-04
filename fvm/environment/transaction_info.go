package environment

import (
	"github.com/onflow/cadence/runtime"

	"github.com/onflow/flow-go/fvm/errors"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/trace"
)

type TransactionInfoParams struct {
	TxIndex uint32
	TxId    flow.Identifier
	TxBody  *flow.TransactionBody

	TransactionFeesEnabled bool
	LimitAccountStorage    bool
}

func DefaultTransactionInfoParams() TransactionInfoParams {
	// NOTE: TxIndex, TxId and TxBody are populated by NewTransactionEnv rather
	// than by Context.
	return TransactionInfoParams{
		TransactionFeesEnabled: false,
		LimitAccountStorage:    false,
	}
}

// TransactionInfo exposes information associated with the executing
// transaction.
//
// Note that scripts have no associated transaction information, but must expose
// the API in compliance with the runtime environment interface.
type TransactionInfo interface {
	TxIndex() uint32
	TxID() flow.Identifier

	TransactionFeesEnabled() bool
	LimitAccountStorage() bool

	SigningAccounts() []runtime.Address

	IsServiceAccountAuthorizer() bool

	// Cadence's runtime API.  Note that the script variant will return
	// OperationNotSupportedError.
	GetSigningAccounts() ([]runtime.Address, error)
}

type transactionInfo struct {
	params TransactionInfoParams

	tracer *Tracer

	authorizers                []runtime.Address
	isServiceAccountAuthorizer bool
}

func NewTransactionInfo(
	params TransactionInfoParams,
	tracer *Tracer,
	serviceAccount flow.Address,
) TransactionInfo {

	isServiceAccountAuthorizer := false
	runtimeAddresses := make(
		[]runtime.Address,
		0,
		len(params.TxBody.Authorizers))

	for _, auth := range params.TxBody.Authorizers {
		runtimeAddresses = append(runtimeAddresses, runtime.Address(auth))
		if auth == serviceAccount {
			isServiceAccountAuthorizer = true
		}
	}

	return &transactionInfo{
		params:                     params,
		tracer:                     tracer,
		authorizers:                runtimeAddresses,
		isServiceAccountAuthorizer: isServiceAccountAuthorizer,
	}
}

func (info *transactionInfo) TxIndex() uint32 {
	return info.params.TxIndex
}

func (info *transactionInfo) TxID() flow.Identifier {
	return info.params.TxId
}

func (info *transactionInfo) TransactionFeesEnabled() bool {
	return info.params.TransactionFeesEnabled
}

func (info *transactionInfo) LimitAccountStorage() bool {
	return info.params.LimitAccountStorage
}

func (info *transactionInfo) SigningAccounts() []runtime.Address {
	return info.authorizers
}

func (info *transactionInfo) IsServiceAccountAuthorizer() bool {
	return info.isServiceAccountAuthorizer
}

func (info *transactionInfo) GetSigningAccounts() ([]runtime.Address, error) {
	defer info.tracer.StartExtensiveTracingSpanFromRoot(
		trace.FVMEnvGetSigningAccounts).End()

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

func (NoTransactionInfo) TransactionFeesEnabled() bool {
	return false
}

func (NoTransactionInfo) LimitAccountStorage() bool {
	return false
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
