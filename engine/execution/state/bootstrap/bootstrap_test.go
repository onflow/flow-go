package bootstrap

import (
	"encoding/hex"
	"testing"

	"github.com/onflow/cadence/runtime"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/engine/execution/computation/virtualmachine"
	"github.com/dapperlabs/flow-go/engine/execution/state"
	"github.com/dapperlabs/flow-go/engine/execution/state/delta"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/module/metrics"
	"github.com/dapperlabs/flow-go/storage/ledger"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

func TestGenerateGenesisStateCommitment(t *testing.T) {
	unittest.RunWithTempDir(t, func(dbDir string) {

		metricsCollector := &metrics.NoopCollector{}
		ls, err := ledger.NewMTrieStorage(dbDir, 100, metricsCollector, nil)
		require.NoError(t, err)

		var initialTokenSupply uint64 = 1000000

		newStateCommitment, err := BootstrapLedger(ls, unittest.RootAccountPublicKey, initialTokenSupply)
		require.NoError(t, err)

		if !assert.Equal(t, unittest.GenesisStateCommitment, newStateCommitment) {
			t.Logf("Actual state commitment: %s", hex.EncodeToString(newStateCommitment))
		}

		vm, err := virtualmachine.New(runtime.NewInterpreterRuntime())
		require.NoError(t, err)

		// create view into genesis state
		view := delta.NewView(state.LedgerGetRegister(ls, newStateCommitment))

		// read service account from genesis state
		serviceAcct, err := vm.NewBlockContext(nil).GetAccount(view, flow.RootAddress)
		require.NoError(t, err)

		// service account balance should equal initial token supply
		assert.Equal(t, initialTokenSupply, serviceAcct.Balance)
	})
}
