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

	n := len(addresses)/TransactionStorageLimiterScriptArgumentBatchSize + 1
	for i := 0; i < n; i++ {
		start := i * TransactionStorageLimiterScriptArgumentBatchSize
		end := (i + 1) * TransactionStorageLimiterScriptArgumentBatchSize
		if end > len(addresses) {
			end = len(addresses)
		}

		err := d.batchCheckLimits(env, addresses[start:end], span)
		if err != nil {
			return err
		}
	}
	return nil
}

func (d TransactionStorageLimiter) batchCheckLimits(
	env Environment,
	addresses []flow.Address,
	span opentracing.Span) error {
	commonAddresses := make([]common.Address, len(addresses))
	for i, address := range addresses {
		commonAddresses[i] = common.Address(address)
	}

	result, invokeErr := InvokeAccountsStorageCapacity(env, span, commonAddresses)

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

		usage, err := env.GetStorageUsed(commonAddresses[i])
		if err != nil {
			return fmt.Errorf("storage limit check failed: %w", err)
		}

		if usage > capacity {
			return errors.NewStorageCapacityExceededError(addresses[i], usage, capacity)
		}
	}

	return nil
}
