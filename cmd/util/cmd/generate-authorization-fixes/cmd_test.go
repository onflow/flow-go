package generate_authorization_fixes

import (
	"fmt"
	"testing"

	"github.com/onflow/cadence"
	jsoncdc "github.com/onflow/cadence/encoding/json"
	"github.com/onflow/cadence/runtime/common"
	"github.com/onflow/cadence/runtime/interpreter"
	"github.com/onflow/cadence/runtime/sema"
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

func TestGenerateAuthorizationFixes(t *testing.T) {
	t.Parallel()

	const chainID = flow.Emulator
	chain := chainID.Chain()

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

	const contractCode = `
      access(all) contract Test {

          access(all) entitlement E

          access(all) struct S {}
      }
    `

	deployTX := flow.NewTransactionBody().
		SetScript([]byte(`
          transaction(code: String) {
              prepare(signer: auth(Contracts) &Account) {
                  signer.contracts.add(name: "Test", code: code.utf8)
              }
          }
        `)).
		AddAuthorizer(address).
		AddArgument(jsoncdc.MustEncode(cadence.String(contractCode)))

	runDeployTx := migrations.NewTransactionBasedMigration(
		deployTX,
		chainID,
		log,
		expectedWriteAddresses,
	)
	err = runDeployTx(registersByAccount)
	require.NoError(t, err)

	setupTx := flow.NewTransactionBody().
		SetScript([]byte(fmt.Sprintf(`
              import Test from %s

              transaction {
                  prepare(signer: auth(Storage, Capabilities) &Account) {
                      // Capability 1 was a public, unauthorized capability, which is now authorized.
                      // It should lose its entitlement
                      let cap1 = signer.capabilities.storage.issue<auth(Test.E) &Test.S>(/storage/s)
                      signer.capabilities.publish(cap1, at: /public/s1)

                      // Capability 2 was a public, unauthorized capability, which is now authorized.
                      // It is currently only stored, nested, in storage, and is not published.
                      // It should lose its entitlement
                      let cap2 = signer.capabilities.storage.issue<auth(Test.E) &Test.S>(/storage/s)
                      signer.storage.save([cap2], to: /storage/caps2)

                      // Capability 3 was a private, authorized capability.
                      // It is currently only stored, nested, in storage, and is not published.
                      // It should keep its entitlement
                      let cap3 = signer.capabilities.storage.issue<auth(Test.E) &Test.S>(/storage/s)
                      signer.storage.save([cap3], to: /storage/caps3)

                      // Capability 4 was a private, authorized capability.
                      // It is currently both stored, nested, in storage, and is published.
                      // It should keep its entitlement
                      let cap4 = signer.capabilities.storage.issue<auth(Test.E) &Test.S>(/storage/s)
                      signer.storage.save([cap4], to: /storage/caps4)
                      signer.capabilities.publish(cap4, at: /public/s4)

                      // Capability 5 was a public, unauthorized capability, which is still unauthorized.
                      // It is currently both stored, nested, in storage, and is published.
                      // There is no need to fix it.
                      let cap5 = signer.capabilities.storage.issue<&Test.S>(/storage/s)
                      signer.storage.save([cap5], to: /storage/caps5)
                      signer.capabilities.publish(cap5, at: /public/s5)
                  }
              }
            `,
			address.HexWithPrefix(),
		))).
		AddAuthorizer(address)

	runSetupTx := migrations.NewTransactionBasedMigration(
		setupTx,
		chainID,
		log,
		expectedWriteAddresses,
	)
	err = runSetupTx(registersByAccount)
	require.NoError(t, err)

	testContractLocation := common.AddressLocation{
		Address: common.Address(address),
		Name:    "Test",
	}

	migratedPublicLinkSet := MigratedPublicLinkSet{
		{
			Address:      common.Address(address),
			CapabilityID: 1,
		}: {},
		{
			Address:      common.Address(address),
			CapabilityID: 2,
		}: {},
		{
			Address:      common.Address(address),
			CapabilityID: 5,
		}: {},
	}

	reporter := &testReportWriter{}

	generator := &AuthorizationFixGenerator{
		registersByAccount:    registersByAccount,
		chainID:               chainID,
		migratedPublicLinkSet: migratedPublicLinkSet,
		reporter:              reporter,
	}
	generator.generateFixesForAllAccounts()

	eTypeID := testContractLocation.TypeID(nil, "Test.E")

	assert.Equal(t,
		[]any{
			fixEntitlementsEntry{
				CapabilityAddress: common.Address(address),
				CapabilityID:      1,
				ReferencedType: interpreter.NewCompositeStaticTypeComputeTypeID(
					nil,
					testContractLocation,
					"Test.S",
				),
				Authorization: newEntitlementSetAuthorizationFromTypeIDs(
					[]common.TypeID{
						eTypeID,
					},
					sema.Conjunction,
				),
			},
			fixEntitlementsEntry{
				CapabilityAddress: common.Address(address),
				CapabilityID:      2,
				ReferencedType: interpreter.NewCompositeStaticTypeComputeTypeID(
					nil,
					testContractLocation,
					"Test.S",
				),
				Authorization: newEntitlementSetAuthorizationFromTypeIDs(
					[]common.TypeID{
						eTypeID,
					},
					sema.Conjunction,
				),
			},
		},
		reporter.entries,
	)
}
