package migration

import (
	"fmt"
	"testing"

	"github.com/onflow/cadence"
	"github.com/onflow/cadence/common"
	"github.com/onflow/cadence/encoding/ccf"
	sdkcrypto "github.com/onflow/flow-go-sdk/crypto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/fvm/accountV2Migration"
	"github.com/onflow/flow-go/integration/internal/emulator"
	"github.com/onflow/flow-go/model/flow"
)

func TestAccountV2Migration(t *testing.T) {
	t.Parallel()

	chain := flow.Emulator.Chain()
	serviceAddress := chain.ServiceAddress()
	serviceAddressIndex, err := chain.IndexFromAddress(serviceAddress)
	require.NoError(t, err)

	accountV2MigrationContractLocation := common.AddressLocation{
		Address: common.Address(serviceAddress),
		Name:    accountV2Migration.ContractName,
	}
	migratedEventTypeID := accountV2MigrationContractLocation.
		TypeID(nil, accountV2Migration.MigratedEventTypeQualifiedIdentifier)

	storage := emulator.NewMemoryStore()

	newOptions := func(accountStorageFormatV2Enabled bool) []emulator.Option {
		return []emulator.Option{
			emulator.WithStorageLimitEnabled(false),
			emulator.WithStore(storage),
			emulator.WithAccountStorageFormatV2Enabled(accountStorageFormatV2Enabled),
		}
	}

	// Ensure that the account V2 migration is disabled,
	// and the service account is in V1 format after bootstrapping

	blockchain, err := emulator.New(newOptions(false)...)
	require.NoError(t, err)

	block, _, err := blockchain.ExecuteAndCommitBlock()
	require.NoError(t, err)

	blockEvents, err := blockchain.GetEventsForBlockIDs(
		string(migratedEventTypeID),
		[]flow.Identifier{block.ID()},
	)
	require.NoError(t, err)
	require.Len(t, blockEvents, 1)
	assert.Empty(t, blockEvents[0].Events)

	checkServiceAccountStorageFormat(t, blockchain, "V1")

	// Enable the storage format V2 migration via system account transaction

	blockchain, err = emulator.New(newOptions(true)...)
	require.NoError(t, err)

	batchSize := serviceAddressIndex + 1

	setAccountV2MigrationBatchSize(t, blockchain, batchSize)

	block, results, err := blockchain.ExecuteAndCommitBlock()
	require.NoError(t, err)

	require.Len(t, results, 1)
	assert.NoError(t, results[0].Error)

	blockEvents, err = blockchain.GetEventsForBlockIDs(
		string(migratedEventTypeID),
		[]flow.Identifier{block.ID()},
	)
	require.NoError(t, err)
	require.Len(t, blockEvents, 1)
	assert.Len(t, blockEvents[0].Events, 1)

	event := blockEvents[0].Events[0]
	assert.Equal(t,
		flow.EventType(migratedEventTypeID),
		event.Type,
	)

	decodedEventPayload, err := ccf.Decode(nil, event.Payload)
	require.NoError(t, err)

	require.IsType(t, cadence.Event{}, decodedEventPayload)

	eventFields := decodedEventPayload.(cadence.Composite).FieldsMappedByName()
	assert.Equal(t, cadence.UInt64(1), eventFields["addressStartIndex"])
	assert.Equal(t, cadence.UInt64(batchSize), eventFields["count"])

	// Ensure the service account is in V2 format after the migration

	checkServiceAccountStorageFormat(t, blockchain, "V2")
}

func sendTransaction(t *testing.T, blockchain *emulator.Blockchain, script string) {
	serviceAddress := blockchain.ServiceKey().Address

	tx := flow.NewTransactionBody().
		SetScript([]byte(script)).
		SetComputeLimit(9999).
		SetProposalKey(serviceAddress, 0, blockchain.ServiceKey().SequenceNumber).
		AddAuthorizer(serviceAddress).
		SetPayer(serviceAddress)

	serviceKey := blockchain.ServiceKey()
	signer, err := sdkcrypto.NewInMemorySigner(
		serviceKey.PrivateKey,
		serviceKey.HashAlgo,
	)
	require.NoError(t, err)

	err = tx.SignEnvelope(serviceAddress, 0, signer.PrivateKey, signer.Hasher)
	assert.NoError(t, err)

	err = blockchain.SendTransaction(tx)
	require.NoError(t, err)
}

func setAccountV2MigrationBatchSize(t *testing.T, blockchain *emulator.Blockchain, batchSize uint64) {
	serviceAddress := blockchain.ServiceKey().Address

	script := fmt.Sprintf(
		`
	        import AccountV2Migration from %[1]s

	        transaction {
	            prepare(signer: auth(Storage) &Account) {
                    log("Enabling account V2 migration")
	                let admin = signer.storage
	                    .borrow<&AccountV2Migration.Admin>(
	                        from: AccountV2Migration.adminStoragePath
	                    )
	                    ?? panic("missing account V2 migration admin resource")
	                admin.setBatchSize(%[2]d)
	            }
	        }
	    `,
		serviceAddress.HexWithPrefix(),
		batchSize,
	)
	sendTransaction(t, blockchain, script)
}

func executeScript(t *testing.T, blockchain *emulator.Blockchain, script string) {
	result, err := blockchain.ExecuteScript([]byte(script), nil)
	require.NoError(t, err)
	require.NoError(t, result.Error)
}

func checkServiceAccountStorageFormat(
	t *testing.T,
	blockchain *emulator.Blockchain,
	storageFormatCaseName string,
) {
	serviceAddress := blockchain.ServiceKey().Address

	executeScript(t,
		blockchain,
		fmt.Sprintf(
			`
                import AccountV2Migration from %[1]s

                access(all) fun main() {
                    let storageFormat = AccountV2Migration.getAccountStorageFormat(address: %[1]s)
                    assert(storageFormat == AccountV2Migration.StorageFormat.%[2]s)
                }
            `,
			serviceAddress.HexWithPrefix(),
			storageFormatCaseName,
		),
	)
}
