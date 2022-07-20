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

const transactionStorageLimiterBatchSize = 100

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

	n := len(addresses)/transactionStorageLimiterBatchSize + 1
	for i := 0; i < n; i++ {
		start := i * transactionStorageLimiterBatchSize
		end := (i + 1) * transactionStorageLimiterBatchSize
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

	accountsStorageCapacity := AccountsStorageCapacityInvocation(env, span)
	result, invokeErr := accountsStorageCapacity(commonAddresses)

	// TODO: Figure out how to handle this error. Currently if a runtime error occurs, storage capacity will be 0.
	// 1. An error will occur if user has removed their FlowToken.Vault -- should this be allowed?
	// 2. There will also be an error in case the accounts balance times megabytesPerFlow constant overflows,
	//		which shouldn't happen unless the the price of storage is reduced at least 100 fold
	// 3. Any other error indicates a bug in our implementation. How can we reliably check the Cadence error?
	if invokeErr != nil {
		return invokeErr
	}

	resultArray, ok := result.(cadence.Array)
	if !ok {
		return fmt.Errorf("TODO err")
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
