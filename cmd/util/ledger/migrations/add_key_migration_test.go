package migrations

import (
	"context"
	"crypto/rand"
	"testing"

	"github.com/onflow/cadence/runtime/common"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go-sdk/crypto"

	"github.com/onflow/flow-go/cmd/util/ledger/util"
	"github.com/onflow/flow-go/cmd/util/ledger/util/registers"
	"github.com/onflow/flow-go/fvm"
	"github.com/onflow/flow-go/fvm/systemcontracts"
	"github.com/onflow/flow-go/model/flow"
)

func TestCoreContractsKeys(t *testing.T) {
	t.Parallel()

	log := zerolog.New(zerolog.NewTestWriter(t))

	// Get the old payloads
	payloads, err := util.PayloadsFromEmulatorSnapshot(snapshotPath)
	require.NoError(t, err)

	registersByAccount, err := registers.NewByAccountFromPayloads(payloads)
	require.NoError(t, err)

	chainID := flow.Emulator
	sc := systemcontracts.SystemContractsForChain(chainID)
	serviceAccountAddress := sc.FlowServiceAccount.Address

	serviceRegisters := registersByAccount.AccountRegisters(string(serviceAccountAddress.Bytes()))

	pk, err := crypto.GeneratePrivateKey(crypto.ECDSA_P256, makeSeed(t))
	require.NoError(t, err)
	expectedKey := pk.PublicKey()
	rwf := &testReportWriterFactory{}

	mig := NewAddKeyMigration(
		chainID,
		expectedKey,
		rwf,
	)
	defer func() {
		err := mig.Close()
		require.NoError(t, err)
	}()

	err = mig.InitMigration(log, registersByAccount, 1)
	require.NoError(t, err)

	ctx := context.Background()
	err = mig.MigrateAccount(ctx, common.Address(serviceAccountAddress), serviceRegisters)
	require.NoError(t, err)

	// Create all the runtime components we need for the migration
	migrationRuntime, err := NewInterpreterMigrationRuntime(
		serviceRegisters,
		chainID,
		InterpreterMigrationRuntimeConfig{},
	)
	require.NoError(t, err)

	// The last key should be the one we added
	keys, err := migrationRuntime.Accounts.GetPublicKeyCount(serviceAccountAddress)
	require.NoError(t, err)

	key, err := migrationRuntime.Accounts.GetPublicKey(serviceAccountAddress, keys-1)
	require.NoError(t, err)

	require.Equal(t, expectedKey.String(), key.PublicKey.String())
	require.Equal(t, fvm.AccountKeyWeightThreshold, key.Weight)
}

func makeSeed(t *testing.T) []byte {
	seed := make([]byte, 32)
	_, err := rand.Read(seed)
	require.NoError(t, err)
	return seed
}
