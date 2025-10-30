package system_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/engine/access/rpc/backend/transactions/system"
	"github.com/onflow/flow-go/fvm/blueprints"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest/fixtures"
)

// TestDefaultSystemCollection tests getting the default system collection returns the correct
// collection and system transaction bodies.
func TestDefaultSystemCollection(t *testing.T) {
	t.Run("scheduled transactions enabled", func(t *testing.T) {
		defaultSystemCollection, err := system.DefaultSystemCollection(flow.Mainnet, true)
		require.NoError(t, err)

		expectedSystemCollection, err := blueprints.SystemCollection(flow.Mainnet.Chain(), nil)
		require.NoError(t, err)

		expectedSystemTx, err := blueprints.SystemChunkTransaction(flow.Mainnet.Chain())
		require.NoError(t, err)

		systemTxID := defaultSystemCollection.SystemTxID()
		systemTx, ok := defaultSystemCollection.ByID(systemTxID)
		require.True(t, ok)

		assert.Equal(t, expectedSystemCollection.Transactions, defaultSystemCollection.Transactions())
		assert.Equal(t, expectedSystemTx, systemTx)
		assert.Equal(t, expectedSystemTx.ID(), systemTxID)

		for _, expectedTx := range expectedSystemCollection.Transactions {
			tx, ok := defaultSystemCollection.ByID(expectedTx.ID())
			assert.True(t, ok)
			assert.Equal(t, expectedTx, tx)
		}
	})

	t.Run("scheduled transactions disabled", func(t *testing.T) {
		defaultSystemCollection, err := system.DefaultSystemCollection(flow.Mainnet, false)
		require.NoError(t, err)

		expectedSystemTx, err := blueprints.SystemChunkTransaction(flow.Mainnet.Chain())
		require.NoError(t, err)

		systemTxID := defaultSystemCollection.SystemTxID()
		systemTx, ok := defaultSystemCollection.ByID(systemTxID)
		require.True(t, ok)

		assert.Equal(t, []*flow.TransactionBody{expectedSystemTx}, defaultSystemCollection.Transactions())
		assert.Equal(t, expectedSystemTx, systemTx)
		assert.Equal(t, expectedSystemTx.ID(), systemTxID)
	})
}

// TestNewSystemCollection tests getting the system collection with scheduled transactions returns
// the correct collection and transaction bodies, including the scheduled transactions.
func TestNewSystemCollection(t *testing.T) {
	g := fixtures.NewGeneratorSuite()
	events := g.PendingExecutionEvents().List(4)
	chainID := g.ChainID()

	systemCollection, err := system.NewSystemCollection(chainID, events)
	require.NoError(t, err)

	expectedSystemCollection, err := blueprints.SystemCollection(chainID.Chain(), events)
	require.NoError(t, err)

	expectedSystemTx, err := blueprints.SystemChunkTransaction(chainID.Chain())
	require.NoError(t, err)

	// scheduled tx + process tx + system chunk tx
	assert.Len(t, systemCollection.Transactions(), 2+len(events))

	systemTxID := systemCollection.SystemTxID()
	systemTx, ok := systemCollection.ByID(systemTxID)
	require.True(t, ok)

	assert.Equal(t, expectedSystemCollection.Transactions, systemCollection.Transactions())
	assert.Equal(t, expectedSystemTx, systemTx)
	assert.Equal(t, expectedSystemTx.ID(), systemTxID)

	for _, expectedTx := range expectedSystemCollection.Transactions {
		tx, ok := systemCollection.ByID(expectedTx.ID())
		assert.True(t, ok)
		assert.Equal(t, expectedTx, tx)
	}
}
