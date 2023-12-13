package reporters_test

import (
	"encoding/hex"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/onflow/cadence"
	jsoncdc "github.com/onflow/cadence/encoding/json"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/cmd/util/ledger/reporters"
	"github.com/onflow/flow-go/fvm"
	"github.com/onflow/flow-go/fvm/storage/state"
	"github.com/onflow/flow-go/fvm/systemcontracts"
	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

func registerIdToLedgerKey(id flow.RegisterID) ledger.Key {
	keyParts := []ledger.KeyPart{
		ledger.NewKeyPart(0, []byte(id.Owner)),
		ledger.NewKeyPart(2, []byte(id.Key)),
	}

	return ledger.NewKey(keyParts)
}

func EntriesToPayloads(updates flow.RegisterEntries) []ledger.Payload {
	ret := make([]ledger.Payload, 0, len(updates))
	for _, entry := range updates {
		key := registerIdToLedgerKey(entry.Key)
		ret = append(ret, *ledger.NewPayload(key, ledger.Value(entry.Value)))
	}

	return ret
}

func TestFungibleTokenTracker(t *testing.T) {

	// bootstrap ledger
	payloads := []ledger.Payload{}
	chain := flow.Testnet.Chain()
	view := state.NewExecutionState(
		reporters.NewStorageSnapshotFromPayload(payloads),
		state.DefaultParameters())

	vm := fvm.NewVirtualMachine()
	opts := []fvm.Option{
		fvm.WithChain(chain),
		fvm.WithAuthorizationChecksEnabled(false),
		fvm.WithSequenceNumberCheckAndIncrementEnabled(false),
	}
	ctx := fvm.NewContext(opts...)
	bootstrapOptions := []fvm.BootstrapProcedureOption{
		fvm.WithTransactionFee(fvm.DefaultTransactionFees),
		fvm.WithAccountCreationFee(fvm.DefaultAccountCreationFee),
		fvm.WithMinimumStorageReservation(fvm.DefaultMinimumStorageReservation),
		fvm.WithStorageMBPerFLOW(fvm.DefaultStorageMBPerFLOW),
		fvm.WithInitialTokenSupply(unittest.GenesisTokenSupply),
	}

	snapshot, _, err := vm.Run(ctx, fvm.Bootstrap(unittest.ServiceAccountPublicKey, bootstrapOptions...), view)
	require.NoError(t, err)

	err = view.Merge(snapshot)
	require.NoError(t, err)

	sc := systemcontracts.SystemContractsForChain(chain.ChainID())

	// deploy wrapper resource
	testContract := fmt.Sprintf(`
	import FungibleToken from 0x%s

	access(all)
	contract WrappedToken {

		access(all)
		resource WrappedVault {

			access(all)
			var vault: @{FungibleToken.Vault}

			init(v: @{FungibleToken.Vault}) {
				self.vault <- v
			}
		}

		access(all)
		fun CreateWrappedVault(inp: @{FungibleToken.Vault}): @WrappedToken.WrappedVault {
			return <-create WrappedVault(v :<- inp)
		}
	}`, sc.FungibleToken.Address.Hex())

	deployingTestContractScript := []byte(fmt.Sprintf(`
	transaction {
		prepare(signer: auth(AddContract) &Account) {
				signer.contracts.add(name: "%s", code: "%s".decodeHex())
		}
	}
	`, "WrappedToken", hex.EncodeToString([]byte(testContract))))

	txBody := flow.NewTransactionBody().
		SetScript(deployingTestContractScript).
		AddAuthorizer(chain.ServiceAddress())

	tx := fvm.Transaction(txBody, 0)
	snapshot, output, err := vm.Run(ctx, tx, view)
	require.NoError(t, err)
	require.NoError(t, output.Err)

	err = view.Merge(snapshot)
	require.NoError(t, err)

	wrapTokenScript := []byte(fmt.Sprintf(
		`
							import FungibleToken from 0x%s
							import FlowToken from 0x%s
							import WrappedToken from 0x%s

							transaction(amount: UFix64) {
								prepare(signer: auth(Storage) &Account) {
									let vaultRef = signer.storage.borrow<auth(FungibleToken.Withdrawable) &FlowToken.Vault>(from: /storage/flowTokenVault)
										?? panic("Could not borrow reference to the owner's Vault!")

									let sentVault <- vaultRef.withdraw(amount: amount)
									let wrappedFlow <- WrappedToken.CreateWrappedVault(inp :<- sentVault)
									signer.storage.save(<-wrappedFlow, to: /storage/wrappedToken)
								}
							}`,
		sc.FungibleToken.Address.Hex(),
		sc.FlowToken.Address.Hex(),
		sc.FlowServiceAccount.Address.Hex(),
	))

	txBody = flow.NewTransactionBody().
		SetScript(wrapTokenScript).
		AddArgument(jsoncdc.MustEncode(cadence.UFix64(105))).
		AddAuthorizer(chain.ServiceAddress())

	tx = fvm.Transaction(txBody, 0)
	snapshot, output, err = vm.Run(ctx, tx, view)
	require.NoError(t, err)
	require.NoError(t, output.Err)

	err = view.Merge(snapshot)
	require.NoError(t, err)

	dir := t.TempDir()
	log := zerolog.Nop()
	reporterFactory := reporters.NewReportFileWriterFactory(dir, log)

	br := reporters.NewFungibleTokenTracker(log, reporterFactory, chain, []string{reporters.FlowTokenTypeID(chain)})
	err = br.Report(
		EntriesToPayloads(view.Finalize().UpdatedRegisters()),
		ledger.State{})
	require.NoError(t, err)

	data, err := os.ReadFile(reporterFactory.Filename(reporters.FungibleTokenTrackerReportPrefix))
	require.NoError(t, err)

	// wrappedToken
	require.True(t, strings.Contains(string(data), `{"path":"storage/wrappedToken/vault","address":"8c5303eaa26202d6","balance":105,"type_id":"A.7e60df042a9c0868.FlowToken.Vault"}`))
	// flowTokenVaults
	require.True(t, strings.Contains(string(data), `{"path":"storage/flowTokenVault","address":"8c5303eaa26202d6","balance":99999999999699895,"type_id":"A.7e60df042a9c0868.FlowToken.Vault"}`))
	require.True(t, strings.Contains(string(data), `{"path":"storage/flowTokenVault","address":"9a0766d93b6608b7","balance":100000,"type_id":"A.7e60df042a9c0868.FlowToken.Vault"}`))
	require.True(t, strings.Contains(string(data), `{"path":"storage/flowTokenVault","address":"7e60df042a9c0868","balance":100000,"type_id":"A.7e60df042a9c0868.FlowToken.Vault"}`))
	require.True(t, strings.Contains(string(data), `{"path":"storage/flowTokenVault","address":"912d5440f7e3769e","balance":100000,"type_id":"A.7e60df042a9c0868.FlowToken.Vault"}`))

	// do not remove this line, see https://github.com/onflow/flow-go/pull/2237
	t.Log("success")
}
