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

	"github.com/onflow/flow-go/cmd/util/ledger/migrations"
	"github.com/onflow/flow-go/cmd/util/ledger/reporters"
	"github.com/onflow/flow-go/fvm"
	"github.com/onflow/flow-go/fvm/programs"
	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestFungibleTokenTracker(t *testing.T) {

	// bootstrap ledger
	payloads := []ledger.Payload{}
	chain := flow.Emulator.Chain()
	view := migrations.NewView(payloads)

	vm := fvm.NewVM()
	opts := []fvm.Option{
		fvm.WithChain(chain),
		fvm.WithTransactionProcessors(
			fvm.NewTransactionInvoker(),
		),
		fvm.WithBlockPrograms(programs.NewEmptyPrograms()),
	}
	ctx := fvm.NewContext(opts...)
	bootstrapOptions := []fvm.BootstrapProcedureOption{
		fvm.WithTransactionFee(fvm.DefaultTransactionFees),
		fvm.WithAccountCreationFee(fvm.DefaultAccountCreationFee),
		fvm.WithMinimumStorageReservation(fvm.DefaultMinimumStorageReservation),
		fvm.WithStorageMBPerFLOW(fvm.DefaultStorageMBPerFLOW),
		fvm.WithInitialTokenSupply(unittest.GenesisTokenSupply),
	}

	err := vm.RunV2(ctx, fvm.Bootstrap(unittest.ServiceAccountPublicKey, bootstrapOptions...), view)
	require.NoError(t, err)

	// deploy wrapper resource
	testContract := fmt.Sprintf(`
	import FungibleToken from 0x%s

	pub contract WrappedToken {
		pub resource WrappedVault {
			pub var vault: @FungibleToken.Vault

			init(v: @FungibleToken.Vault) {
				self.vault <- v
			}
			destroy() {
			  destroy self.vault
			}
		}
		pub fun CreateWrappedVault(inp: @FungibleToken.Vault): @WrappedToken.WrappedVault {
			return <-create WrappedVault(v :<- inp)
		}
	}`, fvm.FungibleTokenAddress(chain))

	deployingTestContractScript := []byte(fmt.Sprintf(`
	transaction {
		prepare(signer: AuthAccount) {
				signer.contracts.add(name: "%s", code: "%s".decodeHex())
		}
	}
	`, "WrappedToken", hex.EncodeToString([]byte(testContract))))

	txBody := flow.NewTransactionBody().
		SetScript(deployingTestContractScript).
		AddAuthorizer(chain.ServiceAddress())

	tx := fvm.Transaction(txBody, 0)
	err = vm.RunV2(ctx, tx, view)
	require.NoError(t, err)
	require.NoError(t, tx.Err)

	wrapTokenScript := []byte(fmt.Sprintf(`
							import FungibleToken from 0x%s
							import FlowToken from 0x%s
							import WrappedToken from 0x%s

							transaction(amount: UFix64) {
								prepare(signer: AuthAccount) {
									let vaultRef = signer.borrow<&FlowToken.Vault>(from: /storage/flowTokenVault)
										?? panic("Could not borrow reference to the owner's Vault!")

									let sentVault <- vaultRef.withdraw(amount: amount)
									let wrappedFlow <- WrappedToken.CreateWrappedVault(inp :<- sentVault)
									signer.save(<-wrappedFlow, to: /storage/wrappedToken)
								}
							}`, fvm.FungibleTokenAddress(chain), fvm.FlowTokenAddress(chain), chain.ServiceAddress()))

	txBody = flow.NewTransactionBody().
		SetScript(wrapTokenScript).
		AddArgument(jsoncdc.MustEncode(cadence.UFix64(105))).
		AddAuthorizer(chain.ServiceAddress())

	tx = fvm.Transaction(txBody, 0)
	err = vm.RunV2(ctx, tx, view)
	require.NoError(t, err)
	require.NoError(t, tx.Err)

	dir := t.TempDir()
	log := zerolog.Nop()
	reporterFactory := reporters.NewReportFileWriterFactory(dir, log)

	br := reporters.NewFungibleTokenTracker(log, reporterFactory, chain, []string{reporters.FlowTokenTypeID(chain)})
	err = br.Report(view.Payloads(), ledger.State{})
	require.NoError(t, err)

	data, err := os.ReadFile(reporterFactory.Filename(reporters.FungibleTokenTrackerReportPrefix))
	require.NoError(t, err)

	// wrappedToken
	require.True(t, strings.Contains(string(data), `{"path":"storage/wrappedToken/vault","address":"f8d6e0586b0a20c7","balance":105,"type_id":"A.f8d6e0586b0a20c7.FlowToken.Vault"}`))
	// flowTokenVaults
	require.True(t, strings.Contains(string(data), `{"path":"storage/flowTokenVault","address":"f8d6e0586b0a20c7","balance":99999999999999895,"type_id":"A.f8d6e0586b0a20c7.FlowToken.Vault"}`))

	// do not remove this line, see https://github.com/onflow/flow-go/pull/2237
	t.Log("success")
}
