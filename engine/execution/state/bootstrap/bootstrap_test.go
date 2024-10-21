package bootstrap

import (
	"encoding/hex"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/fvm"
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

		if !assert.Equal(t, expectedStateCommitment, stateCommitment) {
			t.Logf(
				"Incorrect state commitment: got %s, expected %s",
				hex.EncodeToString(stateCommitment[:]),
				hex.EncodeToString(expectedStateCommitment[:]),
			)
		}
	})
}

func TestBootstrapLedger_ZeroTokenSupply(t *testing.T) {
	expectedStateCommitmentBytes, _ := hex.DecodeString("edc0ffd7e797e2383bc68b694645e6b62b0d996498bcff9b492bf4d426798ec5")
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

		if !assert.Equal(t, expectedStateCommitment, stateCommitment) {
			t.Logf(
				"Incorrect state commitment: got %s, expected %s",
				hex.EncodeToString(stateCommitment[:]),
				hex.EncodeToString(expectedStateCommitment[:]),
			)
		}
	})
}
