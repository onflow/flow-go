package migrations

import (
	"context"
	"errors"
	"fmt"

	"github.com/onflow/atree"
	"github.com/onflow/cadence/runtime"
	"github.com/onflow/cadence/runtime/common"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/cmd/util/ledger/util"
	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/convert"
	"github.com/onflow/flow-go/model/flow"
)

func StorageIDFromRegisterID(registerID flow.RegisterID) atree.StorageID {
	storageID := atree.StorageID{
		Address: atree.Address([]byte(registerID.Owner)),
	}
	copy(storageID.Index[:], registerID.Key[1:])
	return storageID
}

// TODO: use version from atree register inlining (master)
func CheckAccountStorageHealth(
	address common.Address,
	payloads []*ledger.Payload,
	storage *runtime.Storage,
) error {
	// Retrieve all slabs before migration.
	for _, payload := range payloads {
		registerID, _, err := convert.PayloadToRegister(payload)
		if err != nil {
			return fmt.Errorf("failed to convert payload to register: %w", err)
		}

		if !registerID.IsSlabIndex() {
			continue
		}

		storageID := StorageIDFromRegisterID(registerID)

		// Retrieve the slab.
		_, _, err = storage.Retrieve(storageID)
		if err != nil {
			return fmt.Errorf("failed to retrieve slab %s: %w", storageID, err)
		}
	}

	// Load storage map.
	for _, domain := range domains {
		_ = storage.GetStorageMap(address, domain, false)
	}

	return storage.CheckHealth()
}

type FilterUnreferencedSlabsMigration struct {
	log zerolog.Logger
}

var _ AccountBasedMigration = &FilterUnreferencedSlabsMigration{}

func (m *FilterUnreferencedSlabsMigration) InitMigration(
	log zerolog.Logger,
	_ []*ledger.Payload,
	_ int,
) error {
	m.log = log.
		With().
		Str("migration", "filter-unreferenced-slabs").
		Logger()

	return nil
}

func (m *FilterUnreferencedSlabsMigration) MigrateAccount(
	_ context.Context,
	address common.Address,
	oldPayloads []*ledger.Payload,
) ([]*ledger.Payload, error) {

	checkPayloadsOwnership(oldPayloads, address, m.log)

	migrationRuntime, err := NewMigratorRuntime(
		address,
		oldPayloads,
		util.RuntimeInterfaceConfig{},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create migrator runtime: %w", err)
	}

	storage := migrationRuntime.Storage

	newPayloads := oldPayloads

	err = CheckAccountStorageHealth(address, oldPayloads, storage)
	if err != nil {

		// The storage health check failed.
		// This can happen if there are unreferenced root slabs.
		// In this case, we filter out the unreferenced root slabs from the payloads.

		var unreferencedRootSlabsErr runtime.UnreferencedRootSlabsError
		if !errors.As(err, &unreferencedRootSlabsErr) {
			return nil, fmt.Errorf("storage health check failed: %w", err)
		}

		m.log.Warn().
			Err(err).
			Str("account", address.Hex()).
			Msg("filtering unreferenced root slabs")

		// Create a set of unreferenced root slabs.

		unreferencedRootSlabIDs := map[atree.StorageID]struct{}{}
		for _, storageID := range unreferencedRootSlabsErr.UnreferencedRootSlabIDs {
			unreferencedRootSlabIDs[storageID] = struct{}{}
		}

		// Filter out unreferenced root slabs.

		newCount := len(oldPayloads) - len(unreferencedRootSlabIDs)
		newPayloads = make([]*ledger.Payload, 0, newCount)

		filteredPayloads := make([]*ledger.Payload, 0, len(unreferencedRootSlabIDs))

		for _, payload := range oldPayloads {
			registerID, _, err := convert.PayloadToRegister(payload)
			if err != nil {
				return nil, fmt.Errorf("failed to convert payload to register: %w", err)
			}

			// Filter unreferenced slabs.
			if registerID.IsSlabIndex() {
				storageID := StorageIDFromRegisterID(registerID)
				if _, ok := unreferencedRootSlabIDs[storageID]; ok {
					filteredPayloads = append(filteredPayloads, payload)
					continue
				}
			}

			newPayloads = append(newPayloads, payload)
		}

		// TODO: write filtered payloads to a separate file
	}

	return newPayloads, nil
}

func (m *FilterUnreferencedSlabsMigration) Close() error {
	return nil
}
