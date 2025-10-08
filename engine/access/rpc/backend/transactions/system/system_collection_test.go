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

func TestDefaultSystemCollection(t *testing.T) {
	defaultSystemCollection, err := system.DefaultSystemCollection(flow.Mainnet)
	require.NoError(t, err)

	expectedSystemCollection, err := blueprints.SystemCollection(flow.Mainnet.Chain(), nil)
	require.NoError(t, err)

	expectedSystemTx, err := blueprints.SystemChunkTransaction(flow.Mainnet.Chain())
	require.NoError(t, err)

	assert.Equal(t, expectedSystemCollection.Transactions, defaultSystemCollection.Transactions())
	assert.Equal(t, expectedSystemTx, defaultSystemCollection.SystemTx())
	assert.Equal(t, expectedSystemTx.ID(), defaultSystemCollection.SystemTxID())

	for _, expectedTx := range expectedSystemCollection.Transactions {
		tx, ok := defaultSystemCollection.ByID(expectedTx.ID())
		assert.True(t, ok)
		assert.Equal(t, expectedTx, tx)
	}
}

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

	// 4 transactions: process + 2 execute callbacks + system chunk
	assert.Len(t, systemCollection.Transactions(), 2+len(events))

	assert.Equal(t, expectedSystemCollection.Transactions, systemCollection.Transactions())
	assert.Equal(t, expectedSystemTx, systemCollection.SystemTx())
	assert.Equal(t, expectedSystemTx.ID(), systemCollection.SystemTxID())

	for _, expectedTx := range expectedSystemCollection.Transactions {
		tx, ok := systemCollection.ByID(expectedTx.ID())
		assert.True(t, ok)
		assert.Equal(t, expectedTx, tx)
	}
}
