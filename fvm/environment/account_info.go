package environment

import (
	"fmt"

	"github.com/onflow/cadence"
	"github.com/onflow/cadence/common"

	"github.com/onflow/flow-go/fvm/storage/state"
	"github.com/onflow/flow-go/fvm/tracing"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/trace"
)

// AccountInfo exposes various account balance and storage statistics.
type AccountInfo interface {
	// Cadence's runtime APIs.
	GetStorageUsed(runtimeAddress common.Address) (uint64, error)
	GetStorageCapacity(runtimeAddress common.Address) (uint64, error)
	GetAccountBalance(runtimeAddress common.Address) (uint64, error)
	GetAccountAvailableBalance(runtimeAddress common.Address) (uint64, error)

	GetAccount(address flow.Address) (*flow.Account, error)
	GetAccountKeys(address flow.Address) ([]flow.AccountPublicKey, error)
	GetAccountKeyByIndex(address flow.Address, index uint32) (*flow.AccountPublicKey, error)
}

type ParseRestrictedAccountInfo struct {
	txnState state.NestedTransactionPreparer
	impl     AccountInfo
}

func NewParseRestrictedAccountInfo(
	txnState state.NestedTransactionPreparer,
	impl AccountInfo,
) AccountInfo {
	return ParseRestrictedAccountInfo{
		txnState: txnState,
		impl:     impl,
	}
}

func (info ParseRestrictedAccountInfo) GetStorageUsed(
	runtimeAddress common.Address,
) (
	uint64,
	error,
) {
	return parseRestrict1Arg1Ret(
		info.txnState,
		trace.FVMEnvGetStorageUsed,
		info.impl.GetStorageUsed,
		runtimeAddress)
}

func (info ParseRestrictedAccountInfo) GetStorageCapacity(
	runtimeAddress common.Address,
) (
	uint64,
	error,
) {
	return parseRestrict1Arg1Ret(
		info.txnState,
		trace.FVMEnvGetStorageCapacity,
		info.impl.GetStorageCapacity,
		runtimeAddress)
}

func (info ParseRestrictedAccountInfo) GetAccountBalance(
	runtimeAddress common.Address,
) (
	uint64,
	error,
) {
	return parseRestrict1Arg1Ret(
		info.txnState,
		trace.FVMEnvGetAccountBalance,
		info.impl.GetAccountBalance,
		runtimeAddress)
}

func (info ParseRestrictedAccountInfo) GetAccountAvailableBalance(
	runtimeAddress common.Address,
) (
	uint64,
	error,
) {
	return parseRestrict1Arg1Ret(
		info.txnState,
		trace.FVMEnvGetAccountAvailableBalance,
		info.impl.GetAccountAvailableBalance,
		runtimeAddress)
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

func (info ParseRestrictedAccountInfo) GetAccountKeys(
	address flow.Address,
) (
	[]flow.AccountPublicKey,
	error,
) {
	return parseRestrict1Arg1Ret(
		info.txnState,
		trace.FVMEnvGetAccountKeys,
		info.impl.GetAccountKeys,
		address)
}

func (info ParseRestrictedAccountInfo) GetAccountKeyByIndex(
	address flow.Address,
	index uint32,
) (
	*flow.AccountPublicKey,
	error,
) {
	return parseRestrict2Arg1Ret(
		info.txnState,
		trace.FVMEnvGetAccountKey,
		info.impl.GetAccountKeyByIndex,
		address,
		index,
	)
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
	runtimeAddress common.Address,
) (
	uint64,
	error,
) {
	defer info.tracer.StartChildSpan(trace.FVMEnvGetStorageUsed).End()

	err := info.meter.MeterComputation(
		common.ComputationUsage{
			Kind:      ComputationKindGetStorageUsed,
			Intensity: 1,
		},
	)
	if err != nil {
		return 0, fmt.Errorf("get storage used failed: %w", err)
	}

	value, err := info.accounts.GetStorageUsed(
		flow.ConvertAddress(runtimeAddress))
	if err != nil {
		return 0, fmt.Errorf("get storage used failed: %w", err)
	}

	return value, nil
}

// StorageMBUFixToBytesUInt converts the return type of storage capacity which
// is a UFix64 with the unit of megabytes to UInt with the unit of bytes.
func StorageMBUFixToBytesUInt(result cadence.Value) uint64 {
	// Divide the unsigned int by (1e8 (the scale of Fix64) / 1e6 (for mega))
	// to get bytes (rounded down)
	return uint64(result.(cadence.UFix64) / 100)
}

func (info *accountInfo) GetStorageCapacity(
	runtimeAddress common.Address,
) (
	uint64,
	error,
) {
	defer info.tracer.StartChildSpan(trace.FVMEnvGetStorageCapacity).End()

	err := info.meter.MeterComputation(
		common.ComputationUsage{
			Kind:      ComputationKindGetStorageCapacity,
			Intensity: 1,
		},
	)
	if err != nil {
		return 0, fmt.Errorf("get storage capacity failed: %w", err)
	}

	result, invokeErr := info.systemContracts.AccountStorageCapacity(
		flow.ConvertAddress(runtimeAddress))
	if invokeErr != nil {
		return 0, invokeErr
	}

	// Return type is actually a UFix64 with the unit of megabytes so some
	// conversion is necessary divide the unsigned int by (1e8 (the scale of
	// Fix64) / 1e6 (for mega)) to get bytes (rounded down)
	return StorageMBUFixToBytesUInt(result), nil
}

func (info *accountInfo) GetAccountBalance(
	runtimeAddress common.Address,
) (
	uint64,
	error,
) {
	defer info.tracer.StartChildSpan(trace.FVMEnvGetAccountBalance).End()

	err := info.meter.MeterComputation(
		common.ComputationUsage{
			Kind:      ComputationKindGetAccountBalance,
			Intensity: 1,
		},
	)
	if err != nil {
		return 0, fmt.Errorf("get account balance failed: %w", err)
	}

	result, invokeErr := info.systemContracts.AccountBalance(flow.ConvertAddress(runtimeAddress))
	if invokeErr != nil {
		return 0, invokeErr
	}

	return uint64(result.(cadence.UFix64)), nil
}

func (info *accountInfo) GetAccountAvailableBalance(
	runtimeAddress common.Address,
) (
	uint64,
	error,
) {
	defer info.tracer.StartChildSpan(
		trace.FVMEnvGetAccountAvailableBalance).End()

	err := info.meter.MeterComputation(
		common.ComputationUsage{
			Kind:      ComputationKindGetAccountAvailableBalance,
			Intensity: 1,
		},
	)
	if err != nil {
		return 0, fmt.Errorf("get account available balance failed: %w", err)
	}

	result, invokeErr := info.systemContracts.AccountAvailableBalance(flow.ConvertAddress(runtimeAddress))
	if invokeErr != nil {
		return 0, invokeErr
	}

	return uint64(result.(cadence.UFix64)), nil
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
		balance, err := info.GetAccountBalance(
			common.MustBytesToAddress(address.Bytes()))
		if err != nil {
			return nil, err
		}

		account.Balance = balance
	}

	return account, nil
}

func (info *accountInfo) GetAccountKeys(
	address flow.Address,
) (
	[]flow.AccountPublicKey,
	error,
) {
	defer info.tracer.StartChildSpan(trace.FVMEnvGetAccountKeys).End()

	accountKeys, err := info.accounts.GetPublicKeys(address)

	if err != nil {
		return nil, err
	}

	return accountKeys, nil
}

func (info *accountInfo) GetAccountKeyByIndex(
	address flow.Address,
	index uint32,
) (
	*flow.AccountPublicKey,
	error,
) {
	defer info.tracer.StartChildSpan(trace.FVMEnvGetAccountKey).End()

	accountKey, err := info.accounts.GetPublicKey(address, index)

	if err != nil {
		return nil, err
	}

	return &accountKey, nil
}
