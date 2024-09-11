package generate_authorization_fixes

import (
	"errors"
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

	const contractCode = `
      access(all) contract Test {
          access(all) entitlement E1
          access(all) entitlement E2

          access(all) struct S {
              access(E1) fun f1() {}
              access(E2) fun f2() {}
              access(all) fun f3() {}
          }
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
                      // Capability 1 was a public, unauthorized capability.
                      // It should lose its entitlement
                      let cap1 = signer.capabilities.storage.issue<auth(Test.E1, Test.E2) &Test.S>(/storage/s)
                      signer.capabilities.publish(cap1, at: /public/s)

                      // Capability 2 was a public, unauthorized capability, stored nested in storage.
                      // It should lose its entitlement
                      let cap2 = signer.capabilities.storage.issue<auth(Test.E1, Test.E2) &Test.S>(/storage/s)
                      signer.storage.save([cap2], to: /storage/caps2)

                      // Capability 3 was a private, authorized capability, stored nested in storage.
                      // It should keep its entitlement
                      let cap3 = signer.capabilities.storage.issue<auth(Test.E1, Test.E2) &Test.S>(/storage/s)
                      signer.storage.save([cap3], to: /storage/caps3)

	                  // Capability 4 was a capability with unavailable accessible members, stored nested in storage.
	                  // It should keep its entitlement
                      let cap4 = signer.capabilities.storage.issue<auth(Test.E1, Test.E2) &Test.S>(/storage/s)
                      signer.storage.save([cap4], to: /storage/caps4)

                      let cap5 = signer.capabilities.storage.issue<auth(Insert,Mutate,Remove) &{String:String}>(/storage/dict)
                      signer.capabilities.publish(cap5, at: /public/dict)
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

	oldAccessibleMembers := []string{
		"f1",
		"f3",
		"forEachAttachment",
		"getType",
		"isInstance",
		"undefined",
	}

	newAccessibleMembers := []string{
		"f1",
		"f2",
		"f3",
		"forEachAttachment",
		"getType",
		"isInstance",
	}

	testContractLocation := common.AddressLocation{
		Address: common.Address(address),
		Name:    "Test",
	}
	borrowTypeID := testContractLocation.TypeID(nil, "Test.S")

	publicLinkReport := PublicLinkReport{
		{
			Address:    common.Address(address),
			Identifier: "s",
		}: {
			BorrowType:        borrowTypeID,
			AccessibleMembers: oldAccessibleMembers,
		},
		{
			Address:    common.Address(address),
			Identifier: "s2",
		}: {
			BorrowType:        borrowTypeID,
			AccessibleMembers: oldAccessibleMembers,
		},
		{
			Address:    common.Address(address),
			Identifier: "s4",
		}: {
			BorrowType:        borrowTypeID,
			AccessibleMembers: nil,
		},
		{
			Address:    common.Address(address),
			Identifier: "dict",
		}: {
			BorrowType: "{String:String}",
			AccessibleMembers: []string{
				"containsKey",
				"forEachAttachment",
				"forEachKey",
				"getType",
				"insert",
				"isInstance",
				"keys",
				"length",
				"remove",
				"values",
			},
		},
	}

	publicLinkMigrationReport := PublicLinkMigrationReport{
		{
			Address:      common.Address(address),
			CapabilityID: 1,
		}: "s",
		{
			Address:      common.Address(address),
			CapabilityID: 2,
		}: "s2",
		{
			Address:      common.Address(address),
			CapabilityID: 4,
		}: "s4",
		{
			Address:      common.Address(address),
			CapabilityID: 5,
		}: "dict",
	}

	reporter := &testReportWriter{}

	generator := &AuthorizationFixGenerator{
		registersByAccount:        registersByAccount,
		chainID:                   chainID,
		publicLinkReport:          publicLinkReport,
		publicLinkMigrationReport: publicLinkMigrationReport,
		reporter:                  reporter,
	}
	generator.generateFixesForAllAccounts()

	e1TypeID := testContractLocation.TypeID(nil, "Test.E1")
	e2TypeID := testContractLocation.TypeID(nil, "Test.E2")

	assert.Equal(t,
		[]any{
			fixEntitlementsEntry{
				AccountCapabilityID: AccountCapabilityID{
					Address:      common.Address(address),
					CapabilityID: 1,
				},
				ReferencedType: interpreter.NewCompositeStaticTypeComputeTypeID(
					nil,
					testContractLocation,
					"Test.S",
				),
				OldAuthorization: newEntitlementSetAuthorizationFromTypeIDs(
					[]common.TypeID{
						e1TypeID,
						e2TypeID,
					},
					sema.Conjunction,
				),
				NewAuthorization: newEntitlementSetAuthorizationFromTypeIDs(
					[]common.TypeID{
						e1TypeID,
					},
					sema.Conjunction,
				),
				OldAccessibleMembers: oldAccessibleMembers,
				NewAccessibleMembers: newAccessibleMembers,
				UnresolvedMembers: map[string]error{
					"undefined": errors.New("member does not exist"),
				},
			},
			fixEntitlementsEntry{
				AccountCapabilityID: AccountCapabilityID{
					Address:      common.Address(address),
					CapabilityID: 2,
				},
				ReferencedType: interpreter.NewCompositeStaticTypeComputeTypeID(
					nil,
					testContractLocation,
					"Test.S",
				),
				OldAuthorization: newEntitlementSetAuthorizationFromTypeIDs(
					[]common.TypeID{
						e1TypeID,
						e2TypeID,
					},
					sema.Conjunction,
				),
				NewAuthorization: newEntitlementSetAuthorizationFromTypeIDs(
					[]common.TypeID{
						e1TypeID,
					},
					sema.Conjunction,
				),
				OldAccessibleMembers: oldAccessibleMembers,
				NewAccessibleMembers: newAccessibleMembers,
				UnresolvedMembers: map[string]error{
					"undefined": errors.New("member does not exist"),
				},
			},
		},
		reporter.entries,
	)
}
