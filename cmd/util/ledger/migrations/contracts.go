package migrations

import (
	"fmt"

	"github.com/onflow/cadence/runtime/common"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/convert"
	"github.com/onflow/flow-go/model/flow"
)

// NewContractsExtractionMigration returns a migration that extracts the code for contracts from the payloads.
// The given contracts map must be allocated, and gets populated with the code for the contracts found in the payloads.
func NewContractsExtractionMigration(
	contracts map[common.AddressLocation][]byte,
	log zerolog.Logger,
) ledger.Migration {
	return func(payloads []*ledger.Payload) ([]*ledger.Payload, error) {

		log.Info().Msg("extracting contracts from payloads ...")

		for _, payload := range payloads {
			registerID, registerValue, err := convert.PayloadToRegister(payload)
			if err != nil {
				return nil, fmt.Errorf("failed to convert payload to register: %w", err)
			}

			contractName := flow.RegisterIDContractName(registerID)
			if contractName == "" {
				continue
			}

			address, err := common.BytesToAddress([]byte(registerID.Owner))
			if err != nil {
				return nil, fmt.Errorf("failed to convert register owner to address: %w", err)
			}

			addressLocation := common.AddressLocation{
				Address: address,
				Name:    contractName,
			}

			contracts[addressLocation] = registerValue
		}

		log.Info().Msgf("extracted %d contracts from payloads", len(contracts))

		// Return the payloads as-is
		return payloads, nil
	}
}
