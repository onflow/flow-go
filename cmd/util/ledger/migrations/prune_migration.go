package migrations

import (
	"fmt"

	"github.com/onflow/cadence/runtime"
	"github.com/onflow/cadence/runtime/common"
	"github.com/onflow/cadence/runtime/interpreter"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/model/flow"
)

// PruneEmptyMigration removes all the payloads with empty value
// this prunes the trie for values that has been deleted
func PruneEmptyMigration(payload []ledger.Payload) ([]ledger.Payload, error) {
	newPayload := make([]ledger.Payload, 0, len(payload))
	for _, p := range payload {
		if len(p.Value()) > 0 {
			newPayload = append(newPayload, p)
		}
	}
	return newPayload, nil
}

// NewCadence1PruneMigration prunes some values from the service account in the Testnet state
func NewCadence1PruneMigration(chainID flow.ChainID, nWorker int, log zerolog.Logger) ledger.Migration {
	if chainID != flow.Testnet {
		return nil
	}

	serviceAccountAddress := common.Address(chainID.Chain().ServiceAddress())

	migrate := func(storage *runtime.Storage, inter *interpreter.Interpreter) error {

		err := pruneRandomBeaconHistory(storage, inter, log, serviceAccountAddress)
		if err != nil {
			return err
		}

		return nil
	}

	return NewAccountStorageMigration(
		serviceAccountAddress,
		log,
		nWorker,
		migrate,
	)
}

func pruneRandomBeaconHistory(
	storage *runtime.Storage,
	inter *interpreter.Interpreter,
	log zerolog.Logger,
	serviceAccountAddress common.Address,
) error {

	log.Info().Msgf("pruning RandomBeaconHistory in service account %s", serviceAccountAddress)

	contracts := storage.GetStorageMap(serviceAccountAddress, runtime.StorageDomainContract, false)
	if contracts == nil {
		return fmt.Errorf("failed to get contracts storage map")
	}

	randomBeaconHistory, ok := contracts.ReadValue(
		nil,
		interpreter.StringStorageMapKey("RandomBeaconHistory"),
	).(*interpreter.CompositeValue)
	if !ok {
		return fmt.Errorf("failed to read RandomBeaconHistory contract")
	}

	randomSourceHistory, ok := randomBeaconHistory.GetField(
		inter,
		interpreter.EmptyLocationRange,
		"randomSourceHistory",
	).(*interpreter.ArrayValue)
	if !ok {
		return fmt.Errorf("failed to read randomSourceHistory field")
	}

	// Remove all but the last value from the randomSourceHistory
	oldCount := randomSourceHistory.Count()
	removalCount := oldCount - 1

	for i := 0; i < removalCount; i++ {
		randomSourceHistory.RemoveWithoutTransfer(
			inter,
			interpreter.EmptyLocationRange,
			// NOTE: always remove the first element
			0,
		)
	}

	// Check
	if randomSourceHistory.Count() != 1 {
		return fmt.Errorf("failed to prune randomSourceHistory")
	}

	log.Info().Msgf(
		"pruned %d entries in RandomBeaconHistory in service account %s",
		removalCount,
		serviceAccountAddress,
	)

	return nil
}
