package environment

import (
	"fmt"

	"github.com/onflow/cadence"
	"github.com/onflow/cadence/runtime/common"

	"github.com/onflow/flow-go/fvm/state"
	"github.com/onflow/flow-go/fvm/tracing"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/trace"
)

// AccountInfo exposes various account balance and storage statistics.
type AccountInfo interface {
	GetStorageUsed(address common.Address) (uint64, error)
	GetStorageCapacity(address common.Address) (uint64, error)
	GetAccountBalance(address common.Address) (uint64, error)
	GetAccountAvailableBalance(address common.Address) (uint64, error)
	GetAccount(address flow.Address) (*flow.Account, error)
}

type ParseRestrictedAccountInfo struct {
	txnState *state.TransactionState
	impl     AccountInfo
}

func NewParseRestrictedAccountInfo(
	txnState *state.TransactionState,
	impl AccountInfo,
) AccountInfo {
	return ParseRestrictedAccountInfo{
		txnState: txnState,
		impl:     impl,
	}
}

func (info ParseRestrictedAccountInfo) GetStorageUsed(
	address common.Address,
) (
	uint64,
	error,
) {
	return parseRestrict1Arg1Ret(
		info.txnState,
		trace.FVMEnvGetStorageUsed,
		info.impl.GetStorageUsed,
		address)
}

func (info ParseRestrictedAccountInfo) GetStorageCapacity(
	address common.Address,
) (
	uint64,
	error,
) {
	return parseRestrict1Arg1Ret(
		info.txnState,
		trace.FVMEnvGetStorageCapacity,
		info.impl.GetStorageCapacity,
		address)
}

func (info ParseRestrictedAccountInfo) GetAccountBalance(
	address common.Address,
) (
	uint64,
	error,
) {
	return parseRestrict1Arg1Ret(
		info.txnState,
		trace.FVMEnvGetAccountBalance,
		info.impl.GetAccountBalance,
		address)
}

func (info ParseRestrictedAccountInfo) GetAccountAvailableBalance(
	address common.Address,
) (
	uint64,
	error,
) {
	return parseRestrict1Arg1Ret(
		info.txnState,
		trace.FVMEnvGetAccountAvailableBalance,
		info.impl.GetAccountAvailableBalance,
		address)
}

func (info ParseRestrictedAccountInfo) GetAccount(
	address flow.Address,
) (
	*flow.Account,
	error,
) {
	return parseRestrict1Arg1Ret(
		info.txnState,
		trace.FVMEnvGetAccount,
		info.impl.GetAccount,
		address)
}

type accountInfo struct {
	tracer tracing.TracerSpan
	meter  Meter

	accounts        Accounts
	systemContracts *SystemContracts

	serviceAccountEnabled bool
}

func NewAccountInfo(
	tracer tracing.TracerSpan,
	meter Meter,
	accounts Accounts,
	systemContracts *SystemContracts,
	serviceAccountEnabled bool,
) AccountInfo {
	return &accountInfo{
		tracer:                tracer,
		meter:                 meter,
		accounts:              accounts,
		systemContracts:       systemContracts,
		serviceAccountEnabled: serviceAccountEnabled,
	}
}

func (info *accountInfo) GetStorageUsed(
	address common.Address,
) (
	uint64,
	error,
) {
	defer info.tracer.StartChildSpan(trace.FVMEnvGetStorageUsed).End()

	err := info.meter.MeterComputation(ComputationKindGetStorageUsed, 1)
	if err != nil {
		return 0, fmt.Errorf("get storage used failed: %w", err)
	}

	value, err := info.accounts.GetStorageUsed(flow.Address(address))
	if err != nil {
		return 0, fmt.Errorf("get storage used failed: %w", err)
	}

	return value, nil
}

// StorageMBUFixToBytesUInt converts the return type of storage capacity which
// is a UFix64 with the unit of megabytes to UInt with the unit of bytes
func StorageMBUFixToBytesUInt(result cadence.Value) uint64 {
	// Divide the unsigned int by (1e8 (the scale of Fix64) / 1e6 (for mega))
	// to get bytes (rounded down)
	return result.ToGoValue().(uint64) / 100
}

func (info *accountInfo) GetStorageCapacity(
	address common.Address,
) (
	uint64,
	error,
) {
	defer info.tracer.StartChildSpan(trace.FVMEnvGetStorageCapacity).End()

	err := info.meter.MeterComputation(ComputationKindGetStorageCapacity, 1)
	if err != nil {
		return 0, fmt.Errorf("get storage capacity failed: %w", err)
	}

	result, invokeErr := info.systemContracts.AccountStorageCapacity(address)
	if invokeErr != nil {
		return 0, invokeErr
	}

	// Return type is actually a UFix64 with the unit of megabytes so some
	// conversion is necessary divide the unsigned int by (1e8 (the scale of
	// Fix64) / 1e6 (for mega)) to get bytes (rounded down)
	return StorageMBUFixToBytesUInt(result), nil
}

func (info *accountInfo) GetAccountBalance(
	address common.Address,
) (
	uint64,
	error,
) {
	defer info.tracer.StartChildSpan(trace.FVMEnvGetAccountBalance).End()

	err := info.meter.MeterComputation(ComputationKindGetAccountBalance, 1)
	if err != nil {
		return 0, fmt.Errorf("get account balance failed: %w", err)
	}

	result, invokeErr := info.systemContracts.AccountBalance(address)
	if invokeErr != nil {
		return 0, invokeErr
	}
	return result.ToGoValue().(uint64), nil
}

func (info *accountInfo) GetAccountAvailableBalance(
	address common.Address,
) (
	uint64,
	error,
) {
	defer info.tracer.StartChildSpan(
		trace.FVMEnvGetAccountAvailableBalance).End()

	err := info.meter.MeterComputation(
		ComputationKindGetAccountAvailableBalance,
		1)
	if err != nil {
		return 0, fmt.Errorf("get account available balance failed: %w", err)
	}

	result, invokeErr := info.systemContracts.AccountAvailableBalance(address)
	if invokeErr != nil {
		return 0, invokeErr
	}
	return result.ToGoValue().(uint64), nil
}

func (info *accountInfo) GetAccount(
	address flow.Address,
) (
	*flow.Account,
	error,
) {
	defer info.tracer.StartChildSpan(trace.FVMEnvGetAccount).End()

	account, err := info.accounts.Get(address)
	if err != nil {
		return nil, err
	}

	if info.serviceAccountEnabled {
		balance, err := info.GetAccountBalance(common.Address(address))
		if err != nil {
			return nil, err
		}

		account.Balance = balance
	}

	return account, nil
}
