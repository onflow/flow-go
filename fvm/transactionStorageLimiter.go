package fvm

import (
	"fmt"

	"github.com/opentracing/opentracing-go"

	"github.com/onflow/cadence"
	"github.com/onflow/cadence/runtime/common"

	errors "github.com/onflow/flow-go/fvm/errors"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/trace"
)

type TransactionStorageLimiter struct{}

func NewTransactionStorageLimiter() TransactionStorageLimiter {
	return TransactionStorageLimiter{}
}

// TransactionStorageLimiterScriptArgumentBatchSize is the number of accounts to check at once.
// Using to many arguments at once might cause problems during InvokeAccountsStorageCapacity.
const TransactionStorageLimiterScriptArgumentBatchSize = 100

func (d TransactionStorageLimiter) CheckLimits(
	env Environment,
	addresses []flow.Address,
) error {
	if !env.Context().LimitAccountStorage {
		return nil
	}

	var span opentracing.Span
	if te, ok := env.(*TransactionEnv); ok && te.isTraceable() {
		span = te.ctx.Tracer.StartSpanFromParent(te.traceSpan, trace.FVMTransactionStorageUsedCheck)
		defer span.Finish()
	}

	commonAddresses := make([]common.Address, len(addresses))
	usages := make([]uint64, len(commonAddresses))
	for i, address := range addresses {
		c := common.Address(address)
		commonAddresses[i] = c
		u, err := env.GetStorageUsed(c)
		if err != nil {
			return fmt.Errorf("storage limit check failed: %w", err)
		}
		usages[i] = u
	}

	for start := 0; start < len(addresses); start += TransactionStorageLimiterScriptArgumentBatchSize {
		end := start + TransactionStorageLimiterScriptArgumentBatchSize
		if end > len(addresses) {
			end = len(addresses)
		}

		err := d.batchCheckLimits(
			env,
			commonAddresses[start:end],
			usages[start:end],
			span,
		)
		if err != nil {
			return err
		}
	}
	return nil
}

func (d TransactionStorageLimiter) batchCheckLimits(
	env Environment,
	addresses []common.Address,
	usage []uint64,
	span opentracing.Span) error {

	result, invokeErr := InvokeAccountsStorageCapacity(env, span, addresses)

	// This error only occurs in case of implementation errors. The InvokeAccountsStorageCapacity
	// already handles cases where the default vault is missing.
	if invokeErr != nil {
		return invokeErr
	}

	// the resultArray elements are in the same order as the addresses and the addresses are deterministically sorted
	resultArray, ok := result.(cadence.Array)
	if !ok {
		return fmt.Errorf("storage limit check failed: AccountsStorageCapacity did not return an array")
	}

	for i, value := range resultArray.Values {
		capacity := storageMBUFixToBytesUInt(value)

		if usage[i] > capacity {
			return errors.NewStorageCapacityExceededError(flow.BytesToAddress(addresses[i].Bytes()), usage[i], capacity)
		}
	}

	return nil
}
