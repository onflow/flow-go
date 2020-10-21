package fvm

import (
	"encoding/binary"
	"fmt"

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
	accounts := state.NewAccounts(ledger, ctx.Chain)

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
		// does it exist?
		exists, err := isExistingAccount(accounts, owner)
		if err != nil {
			return err
		}
		if !exists {
			continue
		}

		// get capacity. Capacity will be 0 if not set yet. This can only be in the case of a bug.
		// It should have been set during account creation
		capacity, err := getStorageRegisterValue(ledger, owner, state.StorageCapacityRegisterName)
		if err != nil {
			return err
		}
		usage, err := getStorageRegisterValue(ledger, owner, state.StorageUsedRegisterName)
		if err != nil {
			return err
		}

		if usage > capacity {
			return &OverStorageCapacityError{
				Address:         flow.HexToAddress(owner),
				StorageUsed:     usage,
				StorageCapacity: capacity,
			}
		}
	}
	return nil
}

func getStorageRegisterValue(ledger state.Ledger, owner, key string) (uint64, error) {
	bValue, err := ledger.Get(owner, "", key)
	if err != nil {
		return 0, err
	}
	if len(bValue) == 0 {
		return 0, nil
	}
	if len(bValue) != 8 {
		panic(fmt.Sprintf("storage size of %s should be 8 bytes", key))
	}
	return binary.LittleEndian.Uint64(bValue), nil
}

func isExistingAccount(accounts *state.Accounts, owner string) (bool, error) {
	ownerBytes := []byte(owner)
	if len(ownerBytes) != flow.AddressLength {
		// not an address
		return false, nil
	}

	accountExists, err := accounts.Exists(flow.BytesToAddress(ownerBytes))
	return accountExists, err
}
