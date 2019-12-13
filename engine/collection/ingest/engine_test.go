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

type mockNode struct {
	engine *ingest.Engine
	net    *stub.Network
	pool   []*flow.Transaction
}

//func newMockNode(hub *stub.Hub) (*mockNode, error) {
//	log := zerolog.New(os.Stderr).Level(zerolog.ErrorLevel)
//
//	pool := make([]*flow.Transaction, 0)
//
//	net := stub.NewNetwork()
//}

// Malformed, incomplete, unsigned, or otherwise invalid transactions should be
// rejected.
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

// Transactions should be routed to the correct cluster.
func TestClusterRouting(t *testing.T) {
	// TODO
}

// Transactions should be ingested to the transaction pool.
func TestTransactionIngestion(t *testing.T) {
	// TODO
}
