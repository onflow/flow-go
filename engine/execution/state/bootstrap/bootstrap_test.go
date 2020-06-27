package bootstrap

import (
	"encoding/hex"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/module/metrics"
	"github.com/dapperlabs/flow-go/storage/ledger"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

func TestGenerateGenesisState(t *testing.T) {
	unittest.RunWithTempDir(t, func(dbDir string) {

		chain := flow.Mainnet.Chain()

		metricsCollector := &metrics.NoopCollector{}
		ls, err := ledger.NewMTrieStorage(dbDir, 100, metricsCollector, nil)
		require.NoError(t, err)

		stateCommitment, err := NewBootstrapper(zerolog.Nop()).BootstrapLedger(
			ls,
			unittest.ServiceAccountPublicKey,
			unittest.GenesisTokenSupply,
			chain,
		)
		require.NoError(t, err)

		if !assert.Equal(t, unittest.GenesisStateCommitment, stateCommitment) {
			t.Logf("Actual state commitment: %s", hex.EncodeToString(stateCommitment))
		}
	})
}

func TestGenerateGenesisState_ZeroTokenSupply(t *testing.T) {
	var expectedStateCommitment, _ = hex.DecodeString("95c75fdc93069dfd119f98988eb05495b75bde8108c7a6549754bd068810ffa9")
	unittest.RunWithTempDir(t, func(dbDir string) {

		chain := flow.Mainnet.Chain()

		metricsCollector := &metrics.NoopCollector{}
		ls, err := ledger.NewMTrieStorage(dbDir, 100, metricsCollector, nil)
		require.NoError(t, err)

		stateCommitment, err := NewBootstrapper(zerolog.Nop()).BootstrapLedger(ls, unittest.ServiceAccountPublicKey, 0, chain)
		require.NoError(t, err)

		if !assert.Equal(t, expectedStateCommitment, stateCommitment) {
			t.Logf("Actual state commitment: %s", hex.EncodeToString(stateCommitment))
		}
	})
}
