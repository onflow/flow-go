package bootstrap

import (
	"encoding/hex"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	completeLedger "github.com/onflow/flow-go/ledger/complete"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestBootstrapLedger(t *testing.T) {
	unittest.RunWithTempDir(t, func(dbDir string) {

		chain := flow.Mainnet.Chain()

		metricsCollector := &metrics.NoopCollector{}
		ls, err := completeLedger.NewLedger(dbDir, 100, metricsCollector, zerolog.Nop(), nil, completeLedger.DefaultPathFinderVersion)
		require.NoError(t, err)

		stateCommitment, err := NewBootstrapper(zerolog.Nop()).BootstrapLedger(
			ls,
			unittest.ServiceAccountPublicKey,
			unittest.GenesisTokenSupply,
			chain,
		)
		require.NoError(t, err)

		expectedStateCommitment := unittest.GenesisStateCommitment

		if !assert.Equal(t, expectedStateCommitment, stateCommitment) {
			t.Logf(
				"Incorrect state commitment: got %s, expected %s",
				hex.EncodeToString(stateCommitment),
				hex.EncodeToString(expectedStateCommitment),
			)
		}
	})
}

func TestBootstrapLedger_ZeroTokenSupply(t *testing.T) {
	var expectedStateCommitment, _ = hex.DecodeString("207ef54cec1605ff02a82d0382fe9a8851971007d2ef4a2e94426fd76cf8f133")

	unittest.RunWithTempDir(t, func(dbDir string) {

		chain := flow.Mainnet.Chain()

		metricsCollector := &metrics.NoopCollector{}
		ls, err := completeLedger.NewLedger(dbDir, 100, metricsCollector, zerolog.Nop(), nil, completeLedger.DefaultPathFinderVersion)
		require.NoError(t, err)

		stateCommitment, err := NewBootstrapper(zerolog.Nop()).BootstrapLedger(
			ls,
			unittest.ServiceAccountPublicKey,
			0,
			chain,
		)
		require.NoError(t, err)

		if !assert.Equal(t, expectedStateCommitment, stateCommitment) {
			t.Logf(
				"Incorrect state commitment: got %s, expected %s",
				hex.EncodeToString(stateCommitment),
				hex.EncodeToString(expectedStateCommitment),
			)
		}
	})
}
