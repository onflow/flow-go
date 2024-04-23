package migrations

import (
	"context"
	"errors"
	"fmt"
	"path"
	"sync"
	"time"

	"github.com/onflow/atree"
	"github.com/onflow/cadence/runtime"
	"github.com/onflow/cadence/runtime/common"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/cmd/util/ledger/reporters"
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

type FilterUnreferencedSlabsMigration struct {
	log              zerolog.Logger
	rw               reporters.ReportWriter
	outputDir        string
	mutex            sync.Mutex
	filteredPayloads []*ledger.Payload
	payloadsFile     string
}

var _ AccountBasedMigration = &FilterUnreferencedSlabsMigration{}

const filterUnreferencedSlabsName = "filter-unreferenced-slabs"

func NewFilterUnreferencedSlabsMigration(
	outputDir string,
	rwf reporters.ReportWriterFactory,
) *FilterUnreferencedSlabsMigration {
	return &FilterUnreferencedSlabsMigration{
		outputDir: outputDir,
		rw:        rwf.ReportWriter(filterUnreferencedSlabsName),
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

	unreferencedSlabIDs := map[atree.StorageID]struct{}{}
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
		Account:      address,
		PayloadCount: len(filteredPayloads),
	})

	m.mergeFilteredPayloads(filteredPayloads)

	// Do NOT report the health check error here.
	// The health check error is only reported if it is not due to unreferenced slabs.
	// If it is due to unreferenced slabs, we filter them out and continue.

	return newPayloads, nil
}

func (m *FilterUnreferencedSlabsMigration) mergeFilteredPayloads(payloads []*ledger.Payload) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	m.filteredPayloads = append(m.filteredPayloads, payloads...)
}

func (m *FilterUnreferencedSlabsMigration) Close() error {
	// close the report writer so it flushes to file
	m.rw.Close()

	err := m.writeFilteredPayloads()
	if err != nil {
		return fmt.Errorf("failed to write filtered payloads to file: %w", err)
	}

	return nil
}

func (m *FilterUnreferencedSlabsMigration) writeFilteredPayloads() error {

	m.payloadsFile = path.Join(
		m.outputDir,
		fmt.Sprintf("filtered_%d.payloads", int32(time.Now().Unix())),
	)

	writtenPayloadCount, err := util.CreatePayloadFile(
		m.log,
		m.payloadsFile,
		m.filteredPayloads,
		nil,
		true,
	)

	if err != nil {
		return fmt.Errorf("failed to write all filtered payloads to file: %w", err)
	}

	if writtenPayloadCount != len(m.filteredPayloads) {
		return fmt.Errorf(
			"failed to write all filtered payloads to file: expected %d, got %d",
			len(m.filteredPayloads),
			writtenPayloadCount,
		)
	}

	return nil
}

type unreferencedSlabs struct {
	Account      common.Address `json:"account"`
	PayloadCount int            `json:"payload_count"`
}
