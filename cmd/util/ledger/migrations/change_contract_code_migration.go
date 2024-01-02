package migrations

import (
	"context"
	"fmt"

	"github.com/rs/zerolog"

	"github.com/onflow/cadence/runtime/common"

	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/convert"
	"github.com/onflow/flow-go/model/flow"
)

type ChangeContractCodeMigration struct {
	log zerolog.Logger

	contracts map[common.Address]map[flow.RegisterID]string
}

var _ AccountBasedMigration = (*ChangeContractCodeMigration)(nil)

func (d *ChangeContractCodeMigration) Close() error {
	if len(d.contracts) > 0 {
		return fmt.Errorf("failed to find all contract registers that need to be changed")
	}

	return nil
}

func (d *ChangeContractCodeMigration) InitMigration(
	log zerolog.Logger,
	_ []*ledger.Payload,
	_ int,
) error {
	d.log = log.
		With().
		Str("migration", "ChangeContractCodeMigration").
		Logger()

	return nil
}

func (d *ChangeContractCodeMigration) MigrateAccount(
	ctx context.Context,
	address common.Address,
	payloads []*ledger.Payload,
) ([]*ledger.Payload, error) {

	contracts, ok := d.contracts[address]

	if !ok {
		// no contracts to change on this address
		return payloads, nil
	}

	for i, payload := range payloads {
		key, err := payload.Key()
		if err != nil {
			return nil, err
		}
		registerID, err := convert.LedgerKeyToRegisterID(key)
		newContract, ok := contracts[registerID]
		if !ok {
			// not a contract register, or
			// not interested in this contract
			continue
		}

		// change contract code
		payloads[i] = ledger.NewPayload(
			key,
			[]byte(newContract),
		)

		// remove contract from list of contracts to change
		// to keep track of which contracts are left to change
		delete(contracts, registerID)
	}

	if len(contracts) > 0 {
		return nil, fmt.Errorf("failed to find all contract registers that need to be changed")
	}

	// remove address from list of addresses to change
	// to keep track of which addresses are left to change
	delete(d.contracts, address)

	return payloads, nil
}

func (d *ChangeContractCodeMigration) ChangeContract(
	address common.Address,
	contractName string,
	newContractCode string,
) *ChangeContractCodeMigration {
	if d.contracts == nil {
		d.contracts = map[common.Address]map[flow.RegisterID]string{}
	}

	if _, ok := d.contracts[address]; !ok {
		d.contracts[address] = map[flow.RegisterID]string{}
	}

	registerID := flow.ContractRegisterID(flow.ConvertAddress(address), contractName)

	d.contracts[address][registerID] = newContractCode

	return d
}
