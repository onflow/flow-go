package migrations

import (
	"context"
	"errors"
	"fmt"

	"github.com/onflow/atree"
	"github.com/onflow/cadence/runtime"
	"github.com/onflow/cadence/runtime/common"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/cmd/util/ledger/reporters"
	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/convert"
	"github.com/onflow/flow-go/model/flow"
)

func StorageIDFromRegisterID(registerID flow.RegisterID) atree.SlabID {
	storageID := atree.NewSlabID(
		atree.Address([]byte(registerID.Owner)),
		atree.SlabIndex([]byte(registerID.Key[1:])),
	)
	return storageID
}

type FilterUnreferencedSlabsMigration struct {
	log zerolog.Logger
	rw  reporters.ReportWriter
}

var _ AccountBasedMigration = &FilterUnreferencedSlabsMigration{}

const filterUnreferencedSlabsName = "filter-unreferenced-slabs"

func NewFilterUnreferencedSlabsMigration(
	rwf reporters.ReportWriterFactory,
) *FilterUnreferencedSlabsMigration {
	return &FilterUnreferencedSlabsMigration{
		rw: rwf.ReportWriter(filterUnreferencedSlabsName),
	}
}

func (m *FilterUnreferencedSlabsMigration) InitMigration(
	log zerolog.Logger,
	_ []*ledger.Payload,
	_ int,
) error {
	m.log = log.
		With().
		Str("migration", filterUnreferencedSlabsName).
		Logger()

	return nil
}

func (m *FilterUnreferencedSlabsMigration) MigrateAccount(
	_ context.Context,
	address common.Address,
	oldPayloads []*ledger.Payload,
) (
	newPayloads []*ledger.Payload,
	err error,
) {
	migrationRuntime, err := NewAtreeRegisterMigratorRuntime(address, oldPayloads)
	if err != nil {
		return nil, fmt.Errorf("failed to create migrator runtime: %w", err)
	}

	storage := migrationRuntime.Storage

	newPayloads = oldPayloads

	err = checkStorageHealth(address, storage, oldPayloads)
	if err == nil {
		return
	}

	// The storage health check failed.
	// This can happen if there are unreferenced root slabs.
	// In this case, we filter out the unreferenced root slabs and all slabs they reference from the payloads.

	var unreferencedRootSlabsErr runtime.UnreferencedRootSlabsError
	if !errors.As(err, &unreferencedRootSlabsErr) {
		return nil, fmt.Errorf("storage health check failed: %w", err)
	}

	m.log.Warn().
		Err(err).
		Str("account", address.Hex()).
		Msg("filtering unreferenced root slabs")

	// Create a set of unreferenced slabs: root slabs, and all slabs they reference.

	unreferencedSlabIDs := map[atree.SlabID]struct{}{}
	for _, rootSlabID := range unreferencedRootSlabsErr.UnreferencedRootSlabIDs {
		unreferencedSlabIDs[rootSlabID] = struct{}{}

		childReferences, _, err := storage.GetAllChildReferences(rootSlabID)
		if err != nil {
			return nil, fmt.Errorf(
				"failed to get all child references for root slab %s: %w",
				rootSlabID,
				err,
			)
		}

		for _, childSlabID := range childReferences {
			unreferencedSlabIDs[childSlabID] = struct{}{}
		}
	}

	// Filter out unreferenced slabs.

	newCount := len(oldPayloads) - len(unreferencedSlabIDs)
	newPayloads = make([]*ledger.Payload, 0, newCount)

	filteredPayloads := make([]*ledger.Payload, 0, len(unreferencedSlabIDs))

	for _, payload := range oldPayloads {
		registerID, _, err := convert.PayloadToRegister(payload)
		if err != nil {
			return nil, fmt.Errorf("failed to convert payload to register: %w", err)
		}

		// Filter unreferenced slabs.
		if registerID.IsSlabIndex() {
			storageID := StorageIDFromRegisterID(registerID)
			if _, ok := unreferencedSlabIDs[storageID]; ok {
				filteredPayloads = append(filteredPayloads, payload)
				continue
			}
		}

		newPayloads = append(newPayloads, payload)
	}

	m.rw.Write(unreferencedSlabs{
		Account:  address,
		Payloads: filteredPayloads,
	})

	// Do NOT report the health check error here.
	// The health check error is only reported if it is not due to unreferenced slabs.
	// If it is due to unreferenced slabs, we filter them out and continue.

	return newPayloads, nil
}

func (m *FilterUnreferencedSlabsMigration) Close() error {
	// close the report writer so it flushes to file
	m.rw.Close()

	return nil
}

type unreferencedSlabs struct {
	Account  common.Address    `json:"account"`
	Payloads []*ledger.Payload `json:"payloads"`
}
