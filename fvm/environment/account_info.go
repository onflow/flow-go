package environment

import (
	"fmt"

	"github.com/onflow/cadence"
	"github.com/onflow/cadence/runtime/common"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/trace"
)

// AccountInfo exposes various account balance and storage statistics.
type AccountInfo struct {
	tracer *Tracer
	meter  Meter

	accounts        Accounts
	systemContracts *SystemContracts
}

func NewAccountInfo(
	tracer *Tracer,
	meter Meter,
	accounts Accounts,
	systemContracts *SystemContracts,
) *AccountInfo {
	return &AccountInfo{
		tracer:          tracer,
		meter:           meter,
		accounts:        accounts,
		systemContracts: systemContracts,
	}
}

func (info *AccountInfo) GetStorageUsed(
	address common.Address,
) (
	uint64,
	error,
) {
	defer info.tracer.StartSpanFromRoot(trace.FVMEnvGetStorageUsed).End()

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

func (info *AccountInfo) GetStorageCapacity(
	address common.Address,
) (
	uint64,
	error,
) {
	defer info.tracer.StartSpanFromRoot(trace.FVMEnvGetStorageCapacity).End()

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

func (info *AccountInfo) GetAccountBalance(
	address common.Address,
) (
	uint64,
	error,
) {
	defer info.tracer.StartSpanFromRoot(trace.FVMEnvGetAccountBalance).End()

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

func (info *AccountInfo) GetAccountAvailableBalance(
	address common.Address,
) (
	uint64,
	error,
) {
	defer info.tracer.StartSpanFromRoot(trace.FVMEnvGetAccountBalance).End()

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
