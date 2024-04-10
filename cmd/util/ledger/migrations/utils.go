package migrations

import (
	"fmt"

	"github.com/onflow/atree"
	"github.com/onflow/cadence/runtime"
	"github.com/onflow/cadence/runtime/common"
	"github.com/onflow/cadence/runtime/stdlib"

	"github.com/onflow/flow-go/fvm/environment"
	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/convert"
	"github.com/onflow/flow-go/model/flow"
)

type AccountsAtreeLedger struct {
	Accounts environment.Accounts
}

func NewAccountsAtreeLedger(accounts environment.Accounts) *AccountsAtreeLedger {
	return &AccountsAtreeLedger{Accounts: accounts}
}

var _ atree.Ledger = &AccountsAtreeLedger{}

func (a *AccountsAtreeLedger) GetValue(owner, key []byte) ([]byte, error) {
	v, err := a.Accounts.GetValue(
		flow.NewRegisterID(
			flow.BytesToAddress(owner),
			string(key)))
	if err != nil {
		return nil, fmt.Errorf("getting value failed: %w", err)
	}
	return v, nil
}

func (a *AccountsAtreeLedger) SetValue(owner, key, value []byte) error {
	err := a.Accounts.SetValue(
		flow.NewRegisterID(
			flow.BytesToAddress(owner),
			string(key)),
		value)
	if err != nil {
		return fmt.Errorf("setting value failed: %w", err)
	}
	return nil
}

func (a *AccountsAtreeLedger) ValueExists(owner, key []byte) (exists bool, err error) {
	v, err := a.GetValue(owner, key)
	if err != nil {
		return false, fmt.Errorf("checking value existence failed: %w", err)
	}

	return len(v) > 0, nil
}

// AllocateSlabIndex allocates new storage index under the owner accounts to store a new register
func (a *AccountsAtreeLedger) AllocateSlabIndex(owner []byte) (atree.SlabIndex, error) {
	v, err := a.Accounts.AllocateSlabIndex(flow.BytesToAddress(owner))
	if err != nil {
		return atree.SlabIndex{}, fmt.Errorf("storage address allocation failed: %w", err)
	}
	return v, nil
}

func checkStorageHealth(
	address common.Address,
	storage *runtime.Storage,
	payloads []*ledger.Payload,
) error {

	for _, payload := range payloads {
		registerID, _, err := convert.PayloadToRegister(payload)
		if err != nil {
			return fmt.Errorf("failed to convert payload to register: %w", err)
		}

		if !registerID.IsSlabIndex() {
			continue
		}

		// Convert the register ID to a storage ID.
		slabID := atree.NewSlabID(
			atree.Address([]byte(registerID.Owner)),
			atree.SlabIndex([]byte(registerID.Key[1:])))

		// Retrieve the slab.
		_, _, err = storage.Retrieve(slabID)
		if err != nil {
			return fmt.Errorf("failed to retrieve slab %s: %w", slabID, err)
		}
	}

	for _, domain := range domains {
		_ = storage.GetStorageMap(address, domain, false)
	}

	return storage.CheckHealth()
}

// convert all domains
var domains = []string{
	common.PathDomainStorage.Identifier(),
	common.PathDomainPrivate.Identifier(),
	common.PathDomainPublic.Identifier(),
	runtime.StorageDomainContract,
	stdlib.InboxStorageDomain,
	stdlib.CapabilityControllerStorageDomain,
}

var domainsLookupMap = map[string]struct{}{
	common.PathDomainStorage.Identifier():    {},
	common.PathDomainPrivate.Identifier():    {},
	common.PathDomainPublic.Identifier():     {},
	runtime.StorageDomainContract:            {},
	stdlib.InboxStorageDomain:                {},
	stdlib.CapabilityControllerStorageDomain: {},
}
