package migrations

import (
	"context"
	"fmt"

	"github.com/fxamacker/cbor/v2"
	"github.com/rs/zerolog"

	"github.com/onflow/cadence/runtime/common"

	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/convert"
	"github.com/onflow/flow-go/model/flow"
)

// DeduplicateContractNamesMigration checks if the contract names have been duplicated and
// removes the duplicate ones.
type DeduplicateContractNamesMigration struct {
	log zerolog.Logger
}

func (d *DeduplicateContractNamesMigration) Close() error {
	return nil
}

func (d *DeduplicateContractNamesMigration) InitMigration(
	log zerolog.Logger,
	_ []*ledger.Payload,
	_ int,
) error {
	d.log = log.
		With().
		Str("migration", "DeduplicateContractNamesMigration").
		Logger()

	return nil
}

func (d *DeduplicateContractNamesMigration) MigrateAccount(
	ctx context.Context,
	address common.Address,
	payloads []*ledger.Payload,
) ([]*ledger.Payload, error) {
	flowAddress := flow.ConvertAddress(address)
	contractNamesID := flow.ContractNamesRegisterID(flowAddress)

	var contractNamesPayload *ledger.Payload
	contractNamesPayloadIndex := 0
	for i, payload := range payloads {
		key, err := payload.Key()
		if err != nil {
			return nil, err
		}
		id, err := convert.LedgerKeyToRegisterID(key)
		if err != nil {
			return nil, err
		}
		if id == contractNamesID {
			contractNamesPayload = payload
			contractNamesPayloadIndex = i
			break
		}
	}
	if contractNamesPayload == nil {
		return payloads, nil
	}

	var contractNames []string
	err := cbor.Unmarshal(contractNamesPayload.Value(), &contractNames)
	if err != nil {
		return nil, fmt.Errorf("failed to get contract names: %w", err)
	}

	var foundDuplicate bool
	i := 1
	for i < len(contractNames) {
		if contractNames[i-1] != contractNames[i] {
			i++
			continue
		}
		// Found duplicate (contactNames[i-1] == contactNames[i])
		// Remove contractNames[i]
		copy(contractNames[i:], contractNames[i+1:])
		contractNames = contractNames[:len(contractNames)-1]
		foundDuplicate = true
	}

	if !foundDuplicate {
		return payloads, nil
	}

	d.log.Info().
		Str("address", address.Hex()).
		Strs("contract_names", contractNames).
		Msg("removing duplicate contract names")

	newContractNames, err := cbor.Marshal(contractNames)
	if err != nil {
		return nil, fmt.Errorf(
			"cannot encode contract names: %s",
			contractNames,
		)
	}

	payloads[contractNamesPayloadIndex] = ledger.NewPayload(convert.RegisterIDToLedgerKey(contractNamesID), newContractNames)
	return payloads, nil

}

var _ AccountBasedMigration = &DeduplicateContractNamesMigration{}
