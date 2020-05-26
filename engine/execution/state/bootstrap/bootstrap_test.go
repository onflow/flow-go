package bootstrap

import (
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/module/metrics"
	"github.com/dapperlabs/flow-go/storage/ledger"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

func TestGenerateGenesisStateCommitment(t *testing.T) {
	unittest.RunWithTempDir(t, func(dbDir string) {

		metricsCollector := &metrics.NoopCollector{}
		ls, err := ledger.NewMTrieStorage(dbDir, 100, metricsCollector, nil)
		require.NoError(t, err)

		newStateCommitment, err := BootstrapLedger(ls, unittest.ServiceAccountPublicKey, unittest.InitialTokenSupply)
		require.NoError(t, err)

		if !assert.Equal(t, unittest.GenesisStateCommitment, newStateCommitment) {
			t.Logf("Actual state commitment: %s", hex.EncodeToString(newStateCommitment))
		}
	})
}
