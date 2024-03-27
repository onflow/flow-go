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
	expectedChangeAddresses map[flow.Address]struct{},
	expectedOriginalAddresses map[flow.Address]struct{},
	logger zerolog.Logger,
) ([]*ledger.Payload, error) {

	newPayloads := make([]*ledger.Payload, 0, len(originalPayloads))

	// Add all new payloads.
	for id, value := range changes {
		delete(originalPayloads, id)
		if len(value) == 0 {
			continue
		}

		if expectedChangeAddresses != nil {
			ownerAddress := flow.BytesToAddress([]byte(id.Owner))

			if _, ok := expectedChangeAddresses[ownerAddress]; !ok {
				// something was changed that does not belong to this account. Log it.
				logger.Error().
					Str("key", id.String()).
					Str("actual_address", ownerAddress.Hex()).
					Interface("expected_addresses", expectedChangeAddresses).
					Hex("value", value).
					Msg("key is part of the change set, but is for a different account")
			}
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

		if expectedOriginalAddresses != nil {
			ownerAddress := flow.BytesToAddress([]byte(id.Owner))

			if _, ok := expectedOriginalAddresses[ownerAddress]; !ok {
				// something was changed that does not belong to this account. Log it.
				logger.Error().
					Str("key", id.String()).
					Str("actual_address", ownerAddress.Hex()).
					Interface("expected_addresses", expectedOriginalAddresses).
					Hex("value", value.Value()).
					Msg("key is part of the original set, but is for a different account")
			}
		}

		newPayloads = append(newPayloads, value)
	}

	return newPayloads, nil
}
