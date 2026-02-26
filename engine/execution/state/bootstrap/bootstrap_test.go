package bootstrap

import (
	"encoding/hex"
	"fmt"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/engine/execution/state"
	"github.com/onflow/flow-go/engine/execution/storehouse"
	"github.com/onflow/flow-go/fvm"
	"github.com/onflow/flow-go/fvm/systemcontracts"
	completeLedger "github.com/onflow/flow-go/ledger/complete"
	"github.com/onflow/flow-go/ledger/complete/wal/fixtures"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestBootstrapLedger(t *testing.T) {
	unittest.RunWithTempDir(t, func(dbDir string) {

		chain := flow.Mainnet.Chain()

		metricsCollector := &metrics.NoopCollector{}
		wal := &fixtures.NoopWAL{}
		ls, err := completeLedger.NewLedger(wal, 100, metricsCollector, zerolog.Nop(), completeLedger.DefaultPathFinderVersion)
		require.NoError(t, err)
		compactor := fixtures.NewNoopCompactor(ls)
		<-compactor.Ready()
		defer func() {
			<-ls.Done()
			<-compactor.Done()
		}()

		stateCommitment, err := NewBootstrapper(zerolog.Nop()).BootstrapLedger(
			ls,
			unittest.ServiceAccountPublicKey,
			chain,
			fvm.WithInitialTokenSupply(unittest.GenesisTokenSupply),
		)
		require.NoError(t, err)

		expectedStateCommitment := unittest.GenesisStateCommitment

		if !assert.Equal(t, fmt.Sprint(expectedStateCommitment), fmt.Sprint(stateCommitment)) {
			t.Logf(
				"Incorrect state commitment: got %s, expected %s",
				hex.EncodeToString(stateCommitment[:]),
				hex.EncodeToString(expectedStateCommitment[:]),
			)
		}
	})
}

func TestBootstrapLedger_ZeroTokenSupply(t *testing.T) {
	expectedStateCommitmentBytes, _ := hex.DecodeString(
		"133252e776891078b18abee23d9c173145a54f7c7488535055ca6b8eb5fd785a",
	)
	expectedStateCommitment, err := flow.ToStateCommitment(expectedStateCommitmentBytes)
	require.NoError(t, err)

	unittest.RunWithTempDir(t, func(dbDir string) {

		chain := flow.Mainnet.Chain()

		metricsCollector := &metrics.NoopCollector{}
		wal := &fixtures.NoopWAL{}
		ls, err := completeLedger.NewLedger(wal, 100, metricsCollector, zerolog.Nop(), completeLedger.DefaultPathFinderVersion)
		require.NoError(t, err)
		compactor := fixtures.NewNoopCompactor(ls)
		<-compactor.Ready()
		defer func() {
			<-ls.Done()
			<-compactor.Done()
		}()

		stateCommitment, err := NewBootstrapper(zerolog.Nop()).BootstrapLedger(
			ls,
			unittest.ServiceAccountPublicKey,
			chain,
		)
		require.NoError(t, err)

		if !assert.Equal(t, fmt.Sprint(expectedStateCommitment), fmt.Sprint(stateCommitment)) {
			t.Logf(
				"Incorrect state commitment: got %s, expected %s",
				hex.EncodeToString(stateCommitment[:]),
				hex.EncodeToString(expectedStateCommitment[:]),
			)
		}
	})
}

// TestBootstrapLedger_EmptyTransaction bootstraps a ledger with:
// - transaction fees
// - storage fees
// - minimum account balance
// - initial token supply
// Then runs an empty transaction to trigger the bookkeeping parts of a transaction:
// - payer has balance to cover the transaction fees check
// - account storage check
// - transaction fee deduction
// This tests that the state commitment has not changed for the bookkeeping parts of the transaction.
func TestBootstrapLedger_EmptyTransaction(t *testing.T) {
	expectedStateCommitmentBytes, _ := hex.DecodeString(
		"cc55e484da7536ba97c1bcc7fc4201cffa0e7f71b18ebd347e9693454a75b184",
	)
	expectedStateCommitment, err := flow.ToStateCommitment(expectedStateCommitmentBytes)
	require.NoError(t, err)

	unittest.RunWithTempDir(t, func(dbDir string) {

		chain := flow.Mainnet.Chain()

		metricsCollector := &metrics.NoopCollector{}
		wal := &fixtures.NoopWAL{}
		ls, err := completeLedger.NewLedger(wal, 100, metricsCollector, zerolog.Nop(), completeLedger.DefaultPathFinderVersion)
		require.NoError(t, err)
		compactor := fixtures.NewNoopCompactor(ls)
		<-compactor.Ready()
		defer func() {
			<-ls.Done()
			<-compactor.Done()
		}()

		stateCommitment, err := NewBootstrapper(zerolog.Nop()).BootstrapLedger(
			ls,
			unittest.ServiceAccountPublicKey,
			chain,
			fvm.WithInitialTokenSupply(unittest.GenesisTokenSupply),
			fvm.WithAccountCreationFee(fvm.DefaultAccountCreationFee),
			fvm.WithMinimumStorageReservation(fvm.DefaultMinimumStorageReservation),
			fvm.WithTransactionFee(fvm.DefaultTransactionFees),
			fvm.WithStorageMBPerFLOW(fvm.DefaultStorageMBPerFLOW),
		)
		require.NoError(t, err)

		storageSnapshot := state.NewLedgerStorageSnapshot(ls, stateCommitment)
		vm := fvm.NewVirtualMachine()

		ctx := fvm.NewContext(
			chain,
			fvm.WithTransactionFeesEnabled(true),
			fvm.WithAccountStorageLimit(true),
			fvm.WithSequenceNumberCheckAndIncrementEnabled(false),
			fvm.WithAuthorizationChecksEnabled(false),
		)

		sc := systemcontracts.SystemContractsForChain(chain.ChainID())

		// create an empty transaction
		txBody, err := flow.NewTransactionBodyBuilder().
			SetScript([]byte(`
				transaction() {
					prepare() {}
					execute {}
				}
			`)).
			SetProposalKey(sc.FlowServiceAccount.Address, 0, 0).
			SetPayer(sc.FlowServiceAccount.Address).
			Build()
		require.NoError(t, err)

		executionSnapshot, output, err := vm.Run(ctx, fvm.Transaction(txBody, 0), storageSnapshot)
		require.NoError(t, err)
		require.NoError(t, output.Err)

		// make sure we have the expected events
		// all of these events are emitted by the fee deduction
		eventNames := make([]string, 0, len(output.Events))
		for _, event := range output.Events {
			eventNames = append(eventNames, string(event.Type))
		}
		expectedEventNames := []string{
			"A.1654653399040a61.FlowToken.TokensWithdrawn",
			"A.f233dcee88fe0abe.FungibleToken.Withdrawn",
			"A.1654653399040a61.FlowToken.TokensDeposited",
			"A.f233dcee88fe0abe.FungibleToken.Deposited",
			"A.f919ee77447b7497.FlowFees.FeesDeducted",
		}
		require.Equal(t, expectedEventNames, eventNames)

		stateCommitment, _, _, err = state.CommitDelta(
			ls,
			executionSnapshot,
			storehouse.NewExecutingBlockSnapshot(storageSnapshot, stateCommitment),
		)
		require.NoError(t, err)

		if !assert.Equal(t, fmt.Sprint(expectedStateCommitment), fmt.Sprint(stateCommitment)) {
			t.Logf(
				"Incorrect state commitment: got %s, expected %s",
				hex.EncodeToString(stateCommitment[:]),
				hex.EncodeToString(expectedStateCommitment[:]),
			)
		}
	})
}
