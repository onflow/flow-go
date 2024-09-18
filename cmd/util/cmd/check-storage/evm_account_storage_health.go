package check_storage

import (
	"bytes"
	"cmp"
	"fmt"
	"slices"

	"golang.org/x/exp/maps"

	"github.com/onflow/atree"

	"github.com/onflow/cadence/runtime"
	"github.com/onflow/cadence/runtime/common"

	"github.com/onflow/flow-go/cmd/util/ledger/util"
	"github.com/onflow/flow-go/cmd/util/ledger/util/registers"
	"github.com/onflow/flow-go/fvm/evm/emulator/state"
	"github.com/onflow/flow-go/model/flow"
)

var (
	compareSlabID = func(a, b atree.SlabID) int {
		return a.Compare(b)
	}

	equalSlabID = func(a, b atree.SlabID) bool {
		return a.Compare(b) == 0
	}
)

// checkEVMAccountStorageHealth checks storage health of cadence-atree
// registers and evm-atree registers in evm account.
func checkEVMAccountStorageHealth(
	address common.Address,
	accountRegisters *registers.AccountRegisters,
) []accountStorageIssue {
	var issues []accountStorageIssue

	ledger := NewReadOnlyLedgerWithAtreeRegisterReadSet(accountRegisters)

	// Check health of cadence-atree registers.
	issues = append(
		issues,
		checkCadenceAtreeRegistersInEVMAccount(address, ledger)...,
	)

	// Check health of evm-atree registers.
	issues = append(
		issues,
		checkEVMAtreeRegistersInEVMAccount(address, ledger)...,
	)

	// Check unreferenced atree registers.
	// If any atree registers are not accessed during health check of
	// cadence-atree and evm-atree registers, these atree registers are
	// unreferenced.
	issues = append(
		issues,
		checkUnreferencedAtreeRegisters(address, ledger, accountRegisters)...,
	)

	return issues
}

// checkCadenceAtreeRegistersInEVMAccount checks health of cadence-atree registers.
func checkCadenceAtreeRegistersInEVMAccount(
	address common.Address,
	ledger atree.Ledger,
) []accountStorageIssue {
	var issues []accountStorageIssue

	storage := runtime.NewStorage(ledger, nil)

	// Load Cadence domains storage map, so atree slab iterator can traverse connected slabs from loaded root slab.
	// NOTE: don't preload all atree slabs in evm account because evm-atree registers require evm-atree decoder.

	for _, domain := range util.StorageMapDomains {
		_ = storage.GetStorageMap(address, domain, false)
	}

	err := storage.CheckHealth()
	if err != nil {
		issues = append(
			issues,
			accountStorageIssue{
				Address: address.Hex(),
				Kind:    storageErrorKindString[cadenceAtreeStorageErrorKind],
				Msg:     fmt.Sprintf("cadence-atree registers health check failed in evm account: %s", err),
			})
	}

	return issues
}

// checkEVMAtreeRegistersInEVMAccount checks health of evm-atree registers.
func checkEVMAtreeRegistersInEVMAccount(
	address common.Address,
	ledger atree.Ledger,
) []accountStorageIssue {
	var issues []accountStorageIssue

	baseStorage := atree.NewLedgerBaseStorage(ledger)

	storage, err := state.NewPersistentSlabStorage(baseStorage)
	if err != nil {
		issues = append(
			issues,
			accountStorageIssue{
				Address: address.Hex(),
				Kind:    storageErrorKindString[evmAtreeStorageErrorKind],
				Msg:     fmt.Sprintf("failed to create atree.PersistentSlabStorage for evm registers: %s", err),
			})
		return issues
	}

	domainSlabIDs := make(map[string]atree.SlabID)

	// Load evm domain root slabs.
	for _, domain := range evmStorageIDKeys {
		rawDomainSlabID, err := ledger.GetValue(address[:], []byte(domain))
		if err != nil {
			issues = append(issues, accountStorageIssue{
				Address: address.Hex(),
				Kind:    storageErrorKindString[evmAtreeStorageErrorKind],
				Msg:     fmt.Sprintf("failed to get evm domain %s raw slab ID: %s", domain, err),
			})
			continue
		}

		if len(rawDomainSlabID) == 0 {
			continue
		}

		domainSlabID, err := atree.NewSlabIDFromRawBytes(rawDomainSlabID)
		if err != nil {
			issues = append(issues, accountStorageIssue{
				Address: address.Hex(),
				Kind:    storageErrorKindString[evmAtreeStorageErrorKind],
				Msg:     fmt.Sprintf("failed to convert evm domain %s raw slab ID %x to atree slab ID: %s", domain, rawDomainSlabID, err),
			})
			continue
		}

		// Retrieve evm domain storage register so slab iterator can traverse connected slabs from root slab.

		_, found, err := storage.Retrieve(domainSlabID)
		if err != nil {
			issues = append(issues, accountStorageIssue{
				Address: address.Hex(),
				Kind:    storageErrorKindString[evmAtreeStorageErrorKind],
				Msg:     fmt.Sprintf("failed to retrieve evm domain %s root slab %s: %s", domain, domainSlabID, err),
			})
			continue
		}
		if !found {
			issues = append(issues, accountStorageIssue{
				Address: address.Hex(),
				Kind:    storageErrorKindString[evmAtreeStorageErrorKind],
				Msg:     fmt.Sprintf("failed to find evm domain %s root slab %s", domain, domainSlabID),
			})
			continue
		}

		domainSlabIDs[domain] = domainSlabID
	}

	if len(domainSlabIDs) == 0 {
		return issues
	}

	// Get evm storage slot slab IDs.
	storageSlotSlabIDs, storageSlotIssues := getStorageSlotRootSlabIDs(
		address,
		domainSlabIDs[state.AccountsStorageIDKey],
		storage)

	issues = append(issues, storageSlotIssues...)

	// Load evm storage slot slabs so slab iterator can traverse connected slabs in storage health check.
	for _, id := range storageSlotSlabIDs {
		_, found, err := storage.Retrieve(id)
		if err != nil {
			issues = append(issues, accountStorageIssue{
				Address: address.Hex(),
				Kind:    storageErrorKindString[evmAtreeStorageErrorKind],
				Msg:     fmt.Sprintf("failed to retrieve evm storage slot %s: %s", id, err),
			})
		}
		if !found {
			issues = append(issues, accountStorageIssue{
				Address: address.Hex(),
				Kind:    storageErrorKindString[evmAtreeStorageErrorKind],
				Msg:     fmt.Sprintf("failed to find evm storage slot %s", id),
			})
		}
	}

	// Expected root slabs include domain root slabs and storage slot root slabs.

	expectedRootSlabIDs := make([]atree.SlabID, 0, len(domainSlabIDs)+len(storageSlotSlabIDs))
	expectedRootSlabIDs = append(expectedRootSlabIDs, maps.Values(domainSlabIDs)...)
	expectedRootSlabIDs = append(expectedRootSlabIDs, storageSlotSlabIDs...)

	issues = append(
		issues,
		// Check storage health of evm-atree registers
		checkHealthWithExpectedRootSlabIDs(address, storage, expectedRootSlabIDs)...,
	)

	return issues
}

// getStorageSlotRootSlabIDs returns evm storage slot root slab IDs.
func getStorageSlotRootSlabIDs(
	address common.Address,
	accountStorageRootSlabID atree.SlabID,
	storage *atree.PersistentSlabStorage,
) ([]atree.SlabID, []accountStorageIssue) {

	if accountStorageRootSlabID == atree.SlabIDUndefined {
		return nil, nil
	}

	var issues []accountStorageIssue

	// Load account storage map
	m, err := atree.NewMapWithRootID(storage, accountStorageRootSlabID, atree.NewDefaultDigesterBuilder())
	if err != nil {
		issues = append(
			issues,
			accountStorageIssue{
				Address: address.Hex(),
				Kind:    storageErrorKindString[evmAtreeStorageErrorKind],
				Msg:     fmt.Sprintf("failed to load evm storage slot %s: %s", accountStorageRootSlabID, err),
			})
		return nil, issues
	}

	storageSlotRootSlabIDs := make(map[atree.SlabID]struct{})

	// Iterate accounts in account storage map to get storage slot collection ID.
	err = m.IterateReadOnly(func(key, value atree.Value) (bool, error) {
		// Check address type
		acctAddress, ok := key.(state.ByteStringValue)
		if !ok {
			issues = append(
				issues,
				accountStorageIssue{
					Address: address.Hex(),
					Kind:    storageErrorKindString[evmAtreeStorageErrorKind],
					Msg:     fmt.Sprintf("expect evm account address as ByteStringValue, got %T", key),
				})
			return true, nil
		}

		// Check encoded account type
		encodedAccount, ok := value.(state.ByteStringValue)
		if !ok {
			issues = append(
				issues,
				accountStorageIssue{
					Address: address.Hex(),
					Kind:    storageErrorKindString[evmAtreeStorageErrorKind],
					Msg:     fmt.Sprintf("expect evm account as ByteStringValue, got %T", key),
				})
			return true, nil
		}

		// Decode account
		acct, err := state.DecodeAccount(encodedAccount.Bytes())
		if err != nil {
			issues = append(
				issues,
				accountStorageIssue{
					Address: address.Hex(),
					Kind:    storageErrorKindString[evmAtreeStorageErrorKind],
					Msg:     fmt.Sprintf("failed to decode account %x in evm account storage: %s", acctAddress.Bytes(), err),
				})
			return true, nil
		}

		storageSlotCollectionID := acct.CollectionID

		if len(storageSlotCollectionID) == 0 {
			return true, nil
		}

		storageSlotSlabID, err := atree.NewSlabIDFromRawBytes(storageSlotCollectionID)
		if err != nil {
			issues = append(
				issues,
				accountStorageIssue{
					Address: address.Hex(),
					Kind:    storageErrorKindString[evmAtreeStorageErrorKind],
					Msg:     fmt.Sprintf("failed to convert storage slot collection ID %x to atree slab ID: %s", storageSlotCollectionID, err),
				})
			return true, nil
		}

		// Check storage slot is not double referenced.
		if _, ok := storageSlotRootSlabIDs[storageSlotSlabID]; ok {
			issues = append(
				issues,
				accountStorageIssue{
					Address: address.Hex(),
					Kind:    storageErrorKindString[evmAtreeStorageErrorKind],
					Msg:     fmt.Sprintf("found storage slot collection %x referenced by multiple accounts", storageSlotCollectionID),
				})
		}

		storageSlotRootSlabIDs[storageSlotSlabID] = struct{}{}

		return true, nil
	})
	if err != nil {
		issues = append(
			issues,
			accountStorageIssue{
				Address: address.Hex(),
				Kind:    storageErrorKindString[evmAtreeStorageErrorKind],
				Msg:     fmt.Sprintf("failed to iterate EVM account storage map: %s", err),
			})
	}

	return maps.Keys(storageSlotRootSlabIDs), nil
}

// checkHealthWithExpectedRootSlabIDs checks atree registers in storage (loaded and connected registers).
func checkHealthWithExpectedRootSlabIDs(
	address common.Address,
	storage *atree.PersistentSlabStorage,
	expectedRootSlabIDs []atree.SlabID,
) []accountStorageIssue {
	var issues []accountStorageIssue

	// Check atree storage health
	rootSlabIDSet, err := atree.CheckStorageHealth(storage, -1)
	if err != nil {
		issues = append(
			issues,
			accountStorageIssue{
				Address: address.Hex(),
				Kind:    storageErrorKindString[evmAtreeStorageErrorKind],
				Msg:     fmt.Sprintf("evm atree storage check failed: %s", err),
			})
		return issues
	}

	// Check if returned root slab IDs match expected root slab IDs.

	rootSlabIDs := maps.Keys(rootSlabIDSet)
	slices.SortFunc(rootSlabIDs, compareSlabID)

	slices.SortFunc(expectedRootSlabIDs, compareSlabID)

	if !slices.EqualFunc(expectedRootSlabIDs, rootSlabIDs, equalSlabID) {
		issues = append(
			issues,
			accountStorageIssue{
				Address: address.Hex(),
				Kind:    storageErrorKindString[evmAtreeStorageErrorKind],
				Msg:     fmt.Sprintf("root slabs %v from storage health check != expected root slabs %v", rootSlabIDs, expectedRootSlabIDs),
			})
	}

	return issues
}

// checkUnreferencedAtreeRegisters checks if all atree registers in account has been read through ledger.
func checkUnreferencedAtreeRegisters(
	address common.Address,
	ledger *ReadOnlyLedgerWithAtreeRegisterReadSet,
	accountRegisters *registers.AccountRegisters,
) []accountStorageIssue {
	var issues []accountStorageIssue

	allAtreeRegisterIDs, err := getAtreeRegisterIDsFromRegisters(accountRegisters)
	if err != nil {
		issues = append(
			issues,
			accountStorageIssue{
				Address: address.Hex(),
				Kind:    storageErrorKindString[otherErrorKind],
				Msg:     fmt.Sprintf("failed to get atree register IDs from account registers: %s", err),
			})
		return issues
	}

	// Check for unreferenced atree slabs by verifing all atree slabs in accountRegisters are read
	// during storage health check for evm-atree and cadence-atree registers.

	if ledger.GetAtreeRegisterReadCount() == len(allAtreeRegisterIDs) {
		return issues
	}

	if ledger.GetAtreeRegisterReadCount() > len(allAtreeRegisterIDs) {
		issues = append(
			issues,
			accountStorageIssue{
				Address: address.Hex(),
				Kind:    storageErrorKindString[otherErrorKind],
				Msg: fmt.Sprintf("%d atree registers was read > %d atree registers in evm account",
					ledger.GetAtreeRegisterReadCount(),
					len(allAtreeRegisterIDs)),
			})
		return issues
	}

	unreferencedAtreeRegisterIDs := make([]flow.RegisterID, 0, len(allAtreeRegisterIDs)-ledger.GetAtreeRegisterReadCount())

	for _, id := range allAtreeRegisterIDs {
		if !ledger.IsAtreeRegisterRead(id) {
			unreferencedAtreeRegisterIDs = append(unreferencedAtreeRegisterIDs, id)
		}
	}

	slices.SortFunc(unreferencedAtreeRegisterIDs, func(a, b flow.RegisterID) int {
		return cmp.Compare(a.Key, b.Key)
	})

	issues = append(issues, accountStorageIssue{
		Address: address.Hex(),
		Kind:    storageErrorKindString[evmAtreeStorageErrorKind],
		Msg: fmt.Sprintf(
			"number of read atree slabs %d != number of atree slabs in storage %d: unreferenced atree registers %v",
			ledger.GetAtreeRegisterReadCount(),
			len(allAtreeRegisterIDs),
			unreferencedAtreeRegisterIDs,
		),
	})

	return issues
}

func getAtreeRegisterIDsFromRegisters(registers registers.Registers) ([]flow.RegisterID, error) {
	registerIDs := make([]flow.RegisterID, 0, registers.Count())

	err := registers.ForEach(func(owner string, key string, _ []byte) error {
		if !flow.IsSlabIndexKey(key) {
			return nil
		}

		registerIDs = append(
			registerIDs,
			flow.NewRegisterID(flow.BytesToAddress([]byte(owner)), key),
		)

		return nil
	})
	if err != nil {
		return nil, err
	}

	return registerIDs, nil
}

func isEVMAccount(owner common.Address) bool {
	return bytes.Equal(owner[:], evmAccount[:])
}

type ReadOnlyLedgerWithAtreeRegisterReadSet struct {
	*registers.ReadOnlyLedger
	atreeRegistersReadSet map[flow.RegisterID]struct{}
}

var _ atree.Ledger = &ReadOnlyLedgerWithAtreeRegisterReadSet{}

func NewReadOnlyLedgerWithAtreeRegisterReadSet(
	accountRegisters *registers.AccountRegisters,
) *ReadOnlyLedgerWithAtreeRegisterReadSet {
	return &ReadOnlyLedgerWithAtreeRegisterReadSet{
		ReadOnlyLedger:        &registers.ReadOnlyLedger{Registers: accountRegisters},
		atreeRegistersReadSet: make(map[flow.RegisterID]struct{}),
	}
}

func (l *ReadOnlyLedgerWithAtreeRegisterReadSet) GetValue(address, key []byte) (value []byte, err error) {
	value, err = l.ReadOnlyLedger.GetValue(address, key)
	if err != nil {
		return nil, err
	}

	if flow.IsSlabIndexKey(string(key)) {
		registerID := flow.NewRegisterID(flow.BytesToAddress(address), string(key))
		l.atreeRegistersReadSet[registerID] = struct{}{}
	}
	return value, nil
}

func (l *ReadOnlyLedgerWithAtreeRegisterReadSet) GetAtreeRegisterReadCount() int {
	return len(l.atreeRegistersReadSet)
}

func (l *ReadOnlyLedgerWithAtreeRegisterReadSet) IsAtreeRegisterRead(id flow.RegisterID) bool {
	_, ok := l.atreeRegistersReadSet[id]
	return ok
}
