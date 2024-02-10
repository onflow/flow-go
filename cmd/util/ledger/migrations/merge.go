package migrations

import (
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/convert"
	"github.com/onflow/flow-go/model/flow"
)

func MergeRegisterChanges(
	originalPayloads map[flow.RegisterID]*ledger.Payload,
	changes map[flow.RegisterID]flow.RegisterValue,
	logger zerolog.Logger,
) ([]*ledger.Payload, error) {

	newPayloads := make([]*ledger.Payload, 0, len(originalPayloads))

	// Add all new payloads.
	for id, value := range changes {
		if len(value) == 0 {
			continue
		}
		key := convert.RegisterIDToLedgerKey(id)
		newPayloads = append(newPayloads, ledger.NewPayload(key, value))
	}

	// Add any old payload that wasn't updated.
	for id, value := range originalPayloads {
		if len(value.Value()) == 0 {
			// This is strange, but we don't want to add empty values. Log it.
			logger.Warn().Msgf("empty value for key %s", id)
			continue
		}

		// If the payload had changed, then it has been added earlier.
		// So skip old payload.
		if _, contains := changes[id]; contains {
			continue
		}

		newPayloads = append(newPayloads, value)
	}

	return newPayloads, nil
}
