package bootstrap

import (
	"encoding/hex"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/module/metrics"
	"github.com/dapperlabs/flow-go/storage/ledger"
	storage "github.com/dapperlabs/flow-go/storage/mock"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

func TestGenerateGenesisState(t *testing.T) {
	unittest.RunWithTempDir(t, func(dbDir string) {

		metricsCollector := &metrics.NoopCollector{}
		ls, err := ledger.NewMTrieStorage(dbDir, 100, metricsCollector, nil)
		require.NoError(t, err)

		stateCommitment, err := BootstrapLedger(
			zerolog.Logger{},
			ls,
			new(storage.Blocks),
			unittest.ServiceAccountPublicKey,
			unittest.GenesisTokenSupply,
		)
		require.NoError(t, err)

		if !assert.Equal(t, unittest.GenesisStateCommitment, stateCommitment) {
			t.Logf("Actual state commitment: %s", hex.EncodeToString(stateCommitment))
		}
	})
}

func TestGenerateGenesisState_ZeroTokenSupply(t *testing.T) {
	var expectedStateCommitment, _ = hex.DecodeString("9e9cb57df31949260de41afa7fe534396f04bfeb54279ad8aef5922a3974cbf0")

	unittest.RunWithTempDir(t, func(dbDir string) {

		metricsCollector := &metrics.NoopCollector{}
		ls, err := ledger.NewMTrieStorage(dbDir, 100, metricsCollector, nil)
		require.NoError(t, err)

		stateCommitment, err := BootstrapLedger(zerolog.Logger{}, ls, new(storage.Blocks), unittest.ServiceAccountPublicKey, 0)
		require.NoError(t, err)

		if !assert.Equal(t, expectedStateCommitment, stateCommitment) {
			t.Logf("Actual state commitment: %s", hex.EncodeToString(stateCommitment))
		}
	})
}
