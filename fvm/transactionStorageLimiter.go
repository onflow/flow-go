package fvm

import (
	"bytes"
	"fmt"

	"github.com/onflow/cadence"
	"github.com/onflow/cadence/runtime/common"
	"golang.org/x/exp/slices"

	"github.com/onflow/flow-go/fvm/environment"
	"github.com/onflow/flow-go/fvm/errors"
	"github.com/onflow/flow-go/fvm/storage/snapshot"
	"github.com/onflow/flow-go/fvm/systemcontracts"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/trace"
)

func addressFromRegisterId(id flow.RegisterID) (flow.Address, bool) {
	if len(id.Owner) != flow.AddressLength {
		return flow.EmptyAddress, false
	}

	return flow.BytesToAddress([]byte(id.Owner)), true
}

type TransactionStorageLimiter struct{}

// CheckStorageLimits checks each account that had its storage written to during a transaction, that its storage used
// is less than its storage capacity.
// Storage used is an FVM register and is easily accessible.
// Storage capacity is calculated by the FlowStorageFees contract from the account's flow balance.
//
// The payers balance is considered to be maxTxFees lower that its actual balance, due to the fact that
// the fee deduction step happens after the storage limit check.
func (limiter TransactionStorageLimiter) CheckStorageLimits(
	ctx Context,
	env environment.Environment,
	snapshot *snapshot.ExecutionSnapshot,
	payer flow.Address,
	maxTxFees uint64,
) error {
	if !env.LimitAccountStorage() {
		return nil
	}

	defer env.StartChildSpan(trace.FVMTransactionStorageUsedCheck).End()

	err := limiter.checkStorageLimits(ctx, env, snapshot, payer, maxTxFees)
	if err != nil {
		return fmt.Errorf("storage limit check failed: %w", err)
	}
	return nil
}

// getStorageCheckAddresses returns a list of addresses to be checked whether
// storage limit is exceeded.  The returned list include addresses of updated
// registers (and the payer's address).
func (limiter TransactionStorageLimiter) getStorageCheckAddresses(
	ctx Context,
	snapshot *snapshot.ExecutionSnapshot,
	payer flow.Address,
	maxTxFees uint64,
) []flow.Address {
	// Multiple updated registers might be from the same address.  We want to
	// duplicated the addresses to reduce check overhead.
	dedup := make(map[flow.Address]struct{}, len(snapshot.WriteSet)+1)
	addresses := make([]flow.Address, 0, len(snapshot.WriteSet)+1)

	// In case the payer is not updated, include it here.  If the maxTxFees is
	// zero, it doesn't matter if the payer is included or not.
	if maxTxFees > 0 {
		dedup[payer] = struct{}{}
		addresses = append(addresses, payer)
	}

	sc := systemcontracts.SystemContractsForChain(ctx.Chain.ChainID())
	for id := range snapshot.WriteSet {
		address, ok := addressFromRegisterId(id)
		if !ok {
			continue
		}

		if limiter.shouldSkipSpecialAddress(ctx, address, sc) {
			continue
		}

		_, ok = dedup[address]
		if ok {
			continue
		}

		dedup[address] = struct{}{}
		addresses = append(addresses, address)
	}

	slices.SortFunc(
		addresses,
		func(a flow.Address, b flow.Address) bool {
			// reverse order to maintain compatibility with previous
			// implementation.
			return bytes.Compare(a[:], b[:]) >= 0
		})
	return addresses
}

// checkStorageLimits checks if the transaction changed the storage of any
// address and exceeded the storage limit.
func (limiter TransactionStorageLimiter) checkStorageLimits(
	ctx Context,
	env environment.Environment,
	snapshot *snapshot.ExecutionSnapshot,
	payer flow.Address,
	maxTxFees uint64,
) error {
	addresses := limiter.getStorageCheckAddresses(ctx, snapshot, payer, maxTxFees)

	usages := make([]uint64, len(addresses))

	for i, address := range addresses {
		ca := common.Address(address)
		u, err := env.GetStorageUsed(ca)
		if err != nil {
			return err
		}

		usages[i] = u
	}

	result, invokeErr := env.AccountsStorageCapacity(
		addresses,
		payer,
		maxTxFees,
	)

	// This error only occurs in case of implementation errors.
	// InvokeAccountsStorageCapacity already handles cases where the default
	// vault is missing.
	if invokeErr != nil {
		return invokeErr
	}

	// The resultArray elements are in the same order as the addresses and the
	// addresses are deterministically sorted.
	resultArray, ok := result.(cadence.Array)
	if !ok {
		return fmt.Errorf("AccountsStorageCapacity did not return an array")
	}

	if len(addresses) != len(resultArray.Values) {
		return fmt.Errorf("number of addresses does not match number of result")
	}

	for i, address := range addresses {
		capacity := environment.StorageMBUFixToBytesUInt(resultArray.Values[i])

		if usages[i] > capacity {
			return errors.NewStorageCapacityExceededError(
				address,
				usages[i],
				capacity)
		}
	}

	return nil
}

// shouldSkipSpecialAddress returns true if the address is a special address where storage
// limits are not enforced.
// This is currently only the EVM storage address. This is a temporary solution.
func (limiter TransactionStorageLimiter) shouldSkipSpecialAddress(
	ctx Context,
	address flow.Address,
	sc *systemcontracts.SystemContracts,
) bool {
	return sc.EVM.Address == address
}
