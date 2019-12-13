package ingest_test

import (
	"errors"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/dapperlabs/flow-go/module/local"
	"github.com/dapperlabs/flow-go/utils/unittest"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/engine/collection/ingest"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/network/stub"
)

// Malformed, incomplete, unsigned, or otherwise invalid transactions should be
// detected.
func TestInvalidTransaction(t *testing.T) {
	node := unittest.IdentityFixture()
	node.Role = flow.RoleCollection

	log := zerolog.New(os.Stderr).Level(zerolog.ErrorLevel)
	hub := stub.NewNetworkHub()

	me, err := local.New(node)
	require.Nil(t, err)

	stub := stub.NewNetwork(nil, me, hub)
	pool := make([]*flow.Transaction, 0)

	engine, err := ingest.New(log, stub, me, pool)
	require.Nil(t, err)

	t.Run("missing field", func(t *testing.T) {
		tx := unittest.TransactionFixture()
		tx.Script = nil

		err := engine.Process(me.NodeID(), &tx)
		if assert.Error(t, err) {
			t.Log(err)
			assert.True(t, errors.Is(err, ingest.ErrIncompleteTransaction{}))
		}
	})

	t.Run("invalid signature", func(t *testing.T) {
		// TODO
	})
}

// Transactions should be routed to the correct cluster and should not be
// routed unnecessarily.
func TestClusterRouting(t *testing.T) {
	const N = 3

	nodes := unittest.IdentityListFixture(N, func(node *flow.Identity) {
		node.Role = flow.RoleCollection
	})

	log := zerolog.New(os.Stderr).Level(zerolog.ErrorLevel)
	hub := stub.NewNetworkHub()

	engines := make([]*ingest.Engine, N)
	pools := make([][]*flow.Transaction, N)

	for i := 0; i < N; i++ {
		me, err := local.New(nodes[i])
		require.Nil(t, err)

		stub := stub.NewNetwork(nil, me, hub)
		pools[i] = make([]*flow.Transaction, 0)

		engines[i], err = ingest.New(log, stub, me, pools[i])
		require.Nil(t, err)
	}

	t.Run("should route transactions for a different cluster", func(t *testing.T) {
		// TODO
	})

	t.Run("should not route transactions for my cluster", func(t *testing.T) {
		// TODO
	})

	t.Run("should not route invalid transactions", func(t *testing.T) {
		// TODO
	})
}

// Transactions should be ingested to the transaction pool.
func TestTransactionIngestion(t *testing.T) {
	// TODO
	t.Run("should ingest valid transaction for my cluster", func(t *testing.T) {
		// TODO
	})

	t.Run("should not ingest invalid transactions for my cluster", func(t *testing.T) {
		// TODO
	})

	t.Run("should not ingest valid transactions for another cluster", func(t *testing.T) {
		// TODO
	})
}
