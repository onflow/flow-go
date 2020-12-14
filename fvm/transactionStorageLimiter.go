package fvm

import (
	"github.com/onflow/flow-go/fvm/state"
	"github.com/onflow/flow-go/model/flow"
)

type TransactionStorageLimiter struct{}

func NewTransactionStorageLimiter() *TransactionStorageLimiter {
	return &TransactionStorageLimiter{}
}

func (d *TransactionStorageLimiter) Process(
	_ *VirtualMachine,
	ctx Context,
	_ *TransactionProcedure,
	ledger state.Ledger,
) error {
	if !ctx.LimitAccountStorage {
		return nil
	}

	storageCapacityResolver := ctx.StorageCapacityResolver
	accounts := state.NewAccounts(ledger)

	registerIds, _ := ledger.RegisterUpdates()

	checked := map[string]struct{}{}
	for _, id := range registerIds {
		owner := id.Owner

		if _, wasChecked := checked[owner]; wasChecked {
			// we already checked this account, move on
			continue
		}
		checked[owner] = struct{}{}

		// is this an address?
		address, isAddress := addressFromString(owner)
		if !isAddress {
			continue
		}
		// does it exist?
		exists, err := accounts.Exists(address)
		if err != nil {
			return err
		}
		if !exists {
			continue
		}

		capacity, err := storageCapacityResolver(ledger, address, ctx)
		if err != nil {
			return err
		}

		usage, err := accounts.GetStorageUsed(address)
		if err != nil {
			return err
		}

		if usage > capacity {
			return &StorageCapacityExceededError{
				Address:         address,
				StorageUsed:     usage,
				StorageCapacity: capacity,
			}
		}
	}
	return nil
}

func addressFromString(owner string) (flow.Address, bool) {
	ownerBytes := []byte(owner)
	if len(ownerBytes) != flow.AddressLength {
		// not an address
		return flow.EmptyAddress, false
	}
	address := flow.BytesToAddress(ownerBytes)
	return address, true
}
