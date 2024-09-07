package generate_authorization_fixes

import (
	"testing"

	"github.com/onflow/cadence/runtime/common"
	"github.com/onflow/cadence/runtime/interpreter"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/cmd/util/ledger/migrations"
	"github.com/onflow/flow-go/cmd/util/ledger/reporters"
	"github.com/onflow/flow-go/cmd/util/ledger/util/registers"
	"github.com/onflow/flow-go/fvm"
	"github.com/onflow/flow-go/fvm/storage/snapshot"
	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/convert"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

func newBootstrapPayloads(
	chainID flow.ChainID,
	bootstrapProcedureOptions ...fvm.BootstrapProcedureOption,
) ([]*ledger.Payload, error) {

	ctx := fvm.NewContext(
		fvm.WithChain(chainID.Chain()),
	)

	vm := fvm.NewVirtualMachine()

	storageSnapshot := snapshot.MapStorageSnapshot{}

	bootstrapProcedure := fvm.Bootstrap(
		unittest.ServiceAccountPublicKey,
		bootstrapProcedureOptions...,
	)

	executionSnapshot, _, err := vm.Run(
		ctx,
		bootstrapProcedure,
		storageSnapshot,
	)
	if err != nil {
		return nil, err
	}

	payloads := make([]*ledger.Payload, 0, len(executionSnapshot.WriteSet))

	for registerID, registerValue := range executionSnapshot.WriteSet {
		payloadKey := convert.RegisterIDToLedgerKey(registerID)
		payload := ledger.NewPayload(payloadKey, registerValue)
		payloads = append(payloads, payload)
	}

	return payloads, nil
}

type testReportWriter struct {
	entries []any
}

func (t *testReportWriter) Write(entry interface{}) {
	t.entries = append(t.entries, entry)
}

func (*testReportWriter) Close() {
	// NO-OP
}

var _ reporters.ReportWriter = &testReportWriter{}

func TestFixAuthorizationsMigrations(t *testing.T) {
	t.Parallel()

	const chainID = flow.Emulator
	chain := chainID.Chain()

	const nWorker = 2

	address, err := chain.AddressAtIndex(1000)
	require.NoError(t, err)

	require.Equal(t, "bf519681cdb888b1", address.Hex())

	log := zerolog.New(zerolog.NewTestWriter(t))

	bootstrapPayloads, err := newBootstrapPayloads(chainID)
	require.NoError(t, err)

	registersByAccount, err := registers.NewByAccountFromPayloads(bootstrapPayloads)
	require.NoError(t, err)

	mr := migrations.NewBasicMigrationRuntime(registersByAccount)

	err = mr.Accounts.Create(nil, address)
	require.NoError(t, err)

	expectedWriteAddresses := map[flow.Address]struct{}{
		address: {},
	}

	err = mr.Commit(expectedWriteAddresses, log)
	require.NoError(t, err)

	tx := flow.NewTransactionBody().
		SetScript([]byte(`
          transaction {
              prepare(signer: auth(Storage, Capabilities) &Account) {
                  // Capability 1 was a public, unauthorized capability.
                  // It should lose its entitlement
                  let cap1 = signer.capabilities.storage.issue<auth(Insert) &[Int]>(/storage/ints)
                  signer.capabilities.publish(cap1, at: /public/ints)

                  // Capability 2 was a public, unauthorized capability, stored nested in storage.
                  // It should lose its entitlement
                  let cap2 = signer.capabilities.storage.issue<auth(Insert) &[Int]>(/storage/ints)
                  signer.storage.save([cap2], to: /storage/caps2)

                  // Capability 3 was a private, authorized capability, stored nested in storage.
                  // It should keep its entitlement
                  let cap3 = signer.capabilities.storage.issue<auth(Insert) &[Int]>(/storage/ints)
                  signer.storage.save([cap3], to: /storage/caps3)

	              // Capability 4 was a capability with unavailable accessible members, stored nested in storage.
	              // It should keep its entitlement
                  let cap4 = signer.capabilities.storage.issue<auth(Insert) &[Int]>(/storage/ints)
                  signer.storage.save([cap4], to: /storage/caps4)
              }
          }
        `)).
		AddAuthorizer(address)

	setupTx := migrations.NewTransactionBasedMigration(
		tx,
		chainID,
		log,
		expectedWriteAddresses,
	)
	err = setupTx(registersByAccount)
	require.NoError(t, err)

	mr2, err := migrations.NewInterpreterMigrationRuntime(
		registersByAccount,
		chainID,
		migrations.InterpreterMigrationRuntimeConfig{},
	)
	require.NoError(t, err)

	// Capability 1 was a public, unauthorized capability.
	// It should lose its entitlement
	//
	// Capability 2 was a public, unauthorized capability, stored nested in storage.
	// It should lose its entitlement
	//
	// Capability 3 was a private, authorized capability, stored nested in storage.
	// It should keep its entitlement
	//
	// Capability 4 was a capability with unavailable accessible members, stored nested in storage.
	// It should keep its entitlement

	readArrayMembers := []string{
		"concat",
		"contains",
		"filter",
		"firstIndex",
		"getType",
		"isInstance",
		"length",
		"map",
		"slice",
		"toConstantSized",
	}

	publicLinkReport := PublicLinkReport{
		{
			Address:    common.Address(address),
			Identifier: "ints",
		}: {
			BorrowType:        "&[Int]",
			AccessibleMembers: readArrayMembers,
		},
		{
			Address:    common.Address(address),
			Identifier: "ints2",
		}: {
			BorrowType:        "&[Int]",
			AccessibleMembers: readArrayMembers,
		},
		{
			Address:    common.Address(address),
			Identifier: "ints4",
		}: {
			BorrowType:        "&[Int]",
			AccessibleMembers: nil,
		},
	}

	publicLinkMigrationReport := PublicLinkMigrationReport{
		{
			Address:      common.Address(address),
			CapabilityID: 1,
		}: "ints",
		{
			Address:      common.Address(address),
			CapabilityID: 2,
		}: "ints2",
		{
			Address:      common.Address(address),
			CapabilityID: 4,
		}: "ints4",
	}

	reporter := &testReportWriter{}

	generator := &AuthorizationFixGenerator{
		registersByAccount:        registersByAccount,
		mr:                        mr2,
		publicLinkReport:          publicLinkReport,
		publicLinkMigrationReport: publicLinkMigrationReport,
		reporter:                  reporter,
	}
	generator.generateFixesForAllAccounts()

	assert.Equal(t,
		[]any{
			fixEntitlementsEntry{
				AccountCapabilityID: AccountCapabilityID{
					Address:      common.Address(address),
					CapabilityID: 2,
				},
				NewAuthorization: interpreter.UnauthorizedAccess,
			},
			fixEntitlementsEntry{
				AccountCapabilityID: AccountCapabilityID{
					Address:      common.Address(address),
					CapabilityID: 1,
				},
				NewAuthorization: interpreter.UnauthorizedAccess,
			},
		},
		reporter.entries,
	)

}
