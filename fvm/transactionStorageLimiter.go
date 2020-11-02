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
	_ Context,
	_ *TransactionProcedure,
	ledger state.Ledger,
) error {
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
		// does it exist?
		address, exists, err := isExistingAccount(accounts, owner)
		if err != nil {
			return err
		}
		if !exists {
			continue
		}
		
		capacity, err := accounts.GetStorageCapacity(address)
		if err != nil {
			return err
		}
		usage, err := accounts.GetStorageUsed(address)
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

func isExistingAccount(accounts *state.Accounts, owner string) (flow.Address, bool, error) {
	ownerBytes := []byte(owner)
	if len(ownerBytes) != flow.AddressLength {
		// not an address
		return flow.EmptyAddress, false, nil
	}
	flowAddress := flow.BytesToAddress(ownerBytes)
	accountExists, err := accounts.Exists(flowAddress)
	return flowAddress, accountExists, err
}
