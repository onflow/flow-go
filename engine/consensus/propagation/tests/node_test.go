package tests

import (
	"fmt"
	"math/rand"
	"os"
	"path/filepath"

	"github.com/dgraph-io/badger/v2"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"

	"github.com/dapperlabs/flow-go/engine/consensus/propagation"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/module/local"
	"github.com/dapperlabs/flow-go/module/mempool"
	"github.com/dapperlabs/flow-go/network/stub"
	protocol "github.com/dapperlabs/flow-go/protocol/badger"
)

// mockPropagationNode is a mocked node instance for testing propagation engine.
type mockPropagationNode struct {
	// the on-disk key-value store
	db *badger.DB
	// the real engine to be tested
	engine *propagation.Engine
	// a mocked network layer in order for the Hub to route events in memory to a targeted node
	net *stub.Network
	// the state of the engine, exposed in order for tests to assert
	pool *mempool.Mempool
}

// newMockPropagationNode creates a mocked node with a real engine in it, and "plug" the node into a mocked hub.
func newMockPropagationNode(hub *stub.Hub, genesis *flow.Block, nodeIndex int) (*mockPropagationNode, error) {
	if nodeIndex >= len(genesis.NewIdentities) {
		return nil, errors.Errorf("nodeIndex is out of range: %v", nodeIndex)
	}

	id := genesis.NewIdentities[nodeIndex]
	me, err := local.New(id)
	if err != nil {
		return nil, errors.Wrap(err, "could not initialize local")
	}

	// only log error logs
	log := zerolog.New(os.Stderr).Level(zerolog.ErrorLevel)

	pool, err := mempool.New()
	if err != nil {
		return nil, errors.Wrap(err, "could not initialize mempool")
	}

	dir := filepath.Join(os.TempDir(), fmt.Sprintf("flow-test-db-%d", rand.Uint64()))
	db, err := badger.Open(badger.DefaultOptions(dir).WithLogger(nil))
	if err != nil {
		return nil, errors.Wrap(err, "could not initialize db")
	}

	state, err := protocol.NewState(db)
	if err != nil {
		return nil, errors.Wrap(err, "could not initialize protocol state")
	}

	err = state.Mutate().Bootstrap(genesis)
	if err != nil {
		return nil, errors.Wrap(err, "could not bootstrap state")
	}

	net := stub.NewNetwork(state, me, hub)

	engine, err := propagation.New(log, net, state, me, pool)
	if err != nil {
		return nil, err
	}

	return &mockPropagationNode{
		db:     db,
		engine: engine,
		net:    net,
		pool:   pool,
	}, nil
}

func (n *mockPropagationNode) terminate() {
	<-n.engine.Done()
	n.db.Close()
}

func terminate(nodes ...*mockPropagationNode) {
	for _, n := range nodes {
		n.terminate()
	}
}
