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
	chain := flow.Testnet.Chain()
	view := migrations.NewView(payloads)

	rt := fvm.NewInterpreterRuntime()
	vm := fvm.NewVirtualMachine(rt)
	opts := []fvm.Option{
		fvm.WithChain(chain),
		fvm.WithTransactionProcessors(
			fvm.NewTransactionInvoker(zerolog.Nop()),
		),
	}
	ctx := fvm.NewContext(zerolog.Nop(), opts...)
	bootstrapOptions := []fvm.BootstrapProcedureOption{
		fvm.WithTransactionFee(fvm.DefaultTransactionFees),
		fvm.WithAccountCreationFee(fvm.DefaultAccountCreationFee),
		fvm.WithMinimumStorageReservation(fvm.DefaultMinimumStorageReservation),
		fvm.WithStorageMBPerFLOW(fvm.DefaultStorageMBPerFLOW),
		fvm.WithInitialTokenSupply(unittest.GenesisTokenSupply),
	}
	programs := programs.NewEmptyPrograms()

	err := vm.Run(ctx, fvm.Bootstrap(unittest.ServiceAccountPublicKey, bootstrapOptions...), view, programs)
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
	err = vm.Run(ctx, tx, view, programs)
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
	err = vm.Run(ctx, tx, view, programs)
	require.NoError(t, err)
	require.NoError(t, tx.Err)

	dir := t.TempDir()
	log := zerolog.Nop()
	reporterFactory := reporters.NewReportFileWriterFactory(dir, log)

	br := reporters.NewFungibleTokenTracker(log, reporterFactory, chain, []string{reporters.FlowTokenTypeID(chain)})
	err = br.Report(view.Payloads())
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

}
