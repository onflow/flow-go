package migrations

import (
	"context"
	"fmt"

	"github.com/rs/zerolog"

	"github.com/onflow/cadence/runtime/common"
	"github.com/onflow/crypto/hash"
	"github.com/onflow/flow-go-sdk/crypto"
	"github.com/onflow/flow-go/cmd/util/ledger/util/registers"
	"github.com/onflow/flow-go/fvm"
	"github.com/onflow/flow-go/fvm/systemcontracts"
	"github.com/onflow/flow-go/model/flow"
)

// AddKeyMigration adds a new key to the core contracts accounts
type AddKeyMigration struct {
	log zerolog.Logger

	accountsToAddKeyTo map[common.Address]AddKeyMigrationAccountPublicKeyData
	chainID            flow.ChainID
}

var _ AccountBasedMigration = &AddKeyMigration{}

func NewAddKeyMigration(
	chainID flow.ChainID,
	key crypto.PublicKey,
) *AddKeyMigration {

	addresses := make(map[common.Address]AddKeyMigrationAccountPublicKeyData)
	sc := systemcontracts.SystemContractsForChain(chainID).All()

	for _, sc := range sc {
		addresses[common.Address(sc.Address)] = AddKeyMigrationAccountPublicKeyData{
			PublicKey: key,
			HashAlgo:  hash.SHA3_256,
		}
	}

	return &AddKeyMigration{
		accountsToAddKeyTo: addresses,
		chainID:            chainID,
	}
}

type AddKeyMigrationAccountPublicKeyData struct {
	PublicKey crypto.PublicKey
	HashAlgo  hash.HashingAlgorithm
}

func (m *AddKeyMigration) InitMigration(
	log zerolog.Logger,
	_ *registers.ByAccount,
	_ int,
) error {
	m.log = log.With().Str("component", "AddKeyMigration").Logger()
	return nil
}

func (m *AddKeyMigration) Close() error {
	return nil
}

func (m *AddKeyMigration) MigrateAccount(
	_ context.Context,
	address common.Address,
	accountRegisters *registers.AccountRegisters,
) error {

	keyData, ok := m.accountsToAddKeyTo[address]
	if !ok {
		return nil
	}

	// Create all the runtime components we need for the migration
	migrationRuntime, err := NewInterpreterMigrationRuntime(
		accountRegisters,
		m.chainID,
		InterpreterMigrationRuntimeConfig{},
	)

	if err != nil {
		return err
	}

	key := flow.AccountPublicKey{
		PublicKey: keyData.PublicKey,
		SignAlgo:  keyData.PublicKey.Algorithm(),
		HashAlgo:  keyData.HashAlgo,
		Weight:    fvm.AccountKeyWeightThreshold,
	}

	err = migrationRuntime.Accounts.AppendPublicKey(flow.ConvertAddress(address), key)
	if err != nil {
		return err
	}

	// Finalize the transaction
	result, err := migrationRuntime.TransactionState.FinalizeMainTransaction()
	if err != nil {
		return fmt.Errorf("failed to finalize main transaction: %w", err)
	}

	// Merge the changes into the registers
	expectedAddresses := map[flow.Address]struct{}{
		flow.Address(address): {},
	}

	err = registers.ApplyChanges(
		accountRegisters,
		result.WriteSet,
		expectedAddresses,
		m.log,
	)
	if err != nil {
		return fmt.Errorf("failed to apply register changes: %w", err)
	}

	return nil

}
