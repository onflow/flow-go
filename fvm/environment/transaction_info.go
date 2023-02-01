package environment

import (
	"github.com/onflow/cadence/runtime/common"

	"github.com/onflow/flow-go/fvm/errors"
	"github.com/onflow/flow-go/fvm/state"
	"github.com/onflow/flow-go/fvm/tracing"
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

	SigningAccounts() []common.Address

	IsServiceAccountAuthorizer() bool

	// Cadence's runtime API.  Note that the script variant will return
	// OperationNotSupportedError.
	GetSigningAccounts() ([]common.Address, error)
}

type ParseRestrictedTransactionInfo struct {
	txnState *state.TransactionState
	impl     TransactionInfo
}

func NewParseRestrictedTransactionInfo(
	txnState *state.TransactionState,
	impl TransactionInfo,
) TransactionInfo {
	return ParseRestrictedTransactionInfo{
		txnState: txnState,
		impl:     impl,
	}
}

func (info ParseRestrictedTransactionInfo) TxIndex() uint32 {
	return info.impl.TxIndex()
}

func (info ParseRestrictedTransactionInfo) TxID() flow.Identifier {
	return info.impl.TxID()
}

func (info ParseRestrictedTransactionInfo) TransactionFeesEnabled() bool {
	return info.impl.TransactionFeesEnabled()
}

func (info ParseRestrictedTransactionInfo) LimitAccountStorage() bool {
	return info.impl.LimitAccountStorage()
}

func (info ParseRestrictedTransactionInfo) SigningAccounts() []common.Address {
	return info.impl.SigningAccounts()
}

func (info ParseRestrictedTransactionInfo) IsServiceAccountAuthorizer() bool {
	return info.impl.IsServiceAccountAuthorizer()
}

func (info ParseRestrictedTransactionInfo) GetSigningAccounts() (
	[]common.Address,
	error,
) {
	return parseRestrict1Ret(
		info.txnState,
		trace.FVMEnvGetSigningAccounts,
		info.impl.GetSigningAccounts)
}

type transactionInfo struct {
	params TransactionInfoParams

	tracer tracing.TracerSpan

	authorizers                []common.Address
	isServiceAccountAuthorizer bool
}

func NewTransactionInfo(
	params TransactionInfoParams,
	tracer tracing.TracerSpan,
	serviceAccount flow.Address,
) TransactionInfo {

	isServiceAccountAuthorizer := false
	runtimeAddresses := make(
		[]common.Address,
		0,
		len(params.TxBody.Authorizers))

	for _, auth := range params.TxBody.Authorizers {
		runtimeAddresses = append(runtimeAddresses, common.Address(auth))
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

func (info *transactionInfo) SigningAccounts() []common.Address {
	return info.authorizers
}

func (info *transactionInfo) IsServiceAccountAuthorizer() bool {
	return info.isServiceAccountAuthorizer
}

func (info *transactionInfo) GetSigningAccounts() ([]common.Address, error) {
	defer info.tracer.StartExtensiveTracingChildSpan(
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

func (NoTransactionInfo) SigningAccounts() []common.Address {
	return nil
}

func (NoTransactionInfo) IsServiceAccountAuthorizer() bool {
	return false
}

func (NoTransactionInfo) GetSigningAccounts() ([]common.Address, error) {
	return nil, errors.NewOperationNotSupportedError("GetSigningAccounts")
}
