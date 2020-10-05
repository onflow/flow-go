package test

import (
	"fmt"
	"testing"

	"github.com/phayes/freeport"
	"github.com/rs/zerolog"
	mock2 "github.com/stretchr/testify/mock"

	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/module/mock"
	"github.com/onflow/flow-go/network/codec/json"
	"github.com/onflow/flow-go/network/gossip/libp2p"
	"github.com/onflow/flow-go/network/gossip/libp2p/topology"
	protocol "github.com/onflow/flow-go/state/protocol/mock"
	"github.com/onflow/flow-go/utils/unittest"
)

// generateIDs generate flow Identities with a valid port and networking key
func generateIDs(n int) ([]*flow.Identity, []crypto.PrivateKey, error) {
	identities := make([]*flow.Identity, n)
	privateKeys := make([]crypto.PrivateKey, n)
	for i := 0; i < n; i++ {

		identifier := unittest.IdentifierFixture()

		// generate key
		key, err := GenerateNetworkingKey(identifier)
		if err != nil {
			return nil, nil, err
		}
		privateKeys[i] = key

		// generate port
		port, err := freeport.GetFreePort()
		if err != nil {
			return nil, nil, err
		}

		opt := []func(id *flow.Identity){
			func(id *flow.Identity) {
				id.NodeID = identifier
				id.Address = fmt.Sprintf("0.0.0.0:%d", port)
				id.NetworkPubKey = key.PublicKey()
			},
		}

		identities[i] = unittest.IdentityFixture(opt...)
	}
	return identities, privateKeys, nil
}

// generateMiddlewares creates and initializes middleware instances for all the identities
func generateMiddlewares(log zerolog.Logger, identities []*flow.Identity, keys []crypto.PrivateKey) ([]*libp2p.Middleware, error) {
	metrics := metrics.NewNoopCollector()
	mws := make([]*libp2p.Middleware, len(identities))
	for i, id := range identities {
		// creating middleware of nodes
		mw, err := libp2p.NewMiddleware(log,
			json.NewCodec(),
			id.Address,
			id.NodeID,
			keys[i],
			metrics,
			libp2p.DefaultMaxUnicastMsgSize,
			libp2p.DefaultMaxPubSubMsgSize,
			rootBlockID)
		if err != nil {
			return nil, err
		}
		mws[i] = mw
	}
	return mws, nil
}

// generateNetworks generates the network for the given middlewares
func generateNetworks(log zerolog.Logger,
	ids flow.IdentityList,
	mws []*libp2p.Middleware,
	csize int,
	tops []topology.Topology,
	states []*protocol.ReadOnlyState,
	dryrun bool) ([]*libp2p.Network, error) {
	count := len(ids)
	nets := make([]*libp2p.Network, 0)
	metrics := metrics.NewNoopCollector()

	// if no topology is passed in, use the default topology for all networks
	if tops == nil {
		tops = make([]topology.Topology, count)
		for i, id := range ids {
			rpt, err := topology.NewRandPermTopology(id.Role, id.NodeID)
			if err != nil {
				return nil, fmt.Errorf("could not create network: %w", err)
			}
			tops[i] = rpt
		}
	}

	// if no state is passed in, use the default mock state for all networks
	if states == nil {
		states = make([]*protocol.ReadOnlyState, count)
		for i := range states {
			states[i] = generateStateSnapshot(ids)
		}
	}

	for i := 0; i < count; i++ {

		// creates and mocks me
		me := &mock.Local{}
		me.On("NodeID").Return(ids[i].NodeID)
		me.On("NotMeFilter").Return(flow.IdentityFilter(filter.Any))
		me.On("Address").Return(ids[i].Address)

		// create the network
		net, err := libp2p.NewNetwork(log, json.NewCodec(), states[i], me, mws[i], csize, tops[i], metrics)
		if err != nil {
			return nil, fmt.Errorf("could not create network: %w", err)
		}

		nets = append(nets, net)
	}

	// if dryrun then don't actually start the network
	if !dryrun {
		for _, net := range nets {
			<-net.Ready()
		}
	}
	return nets, nil
}

func generateIDsAndMiddlewares(n int, log zerolog.Logger) ([]*flow.Identity, []*libp2p.Middleware, error) {
	ids, keys, err := generateIDs(n)
	if err != nil {
		return nil, nil, err
	}

	mws, err := generateMiddlewares(log, ids, keys)
	if err != nil {
		return nil, nil, err
	}

	return ids, mws, err
}

func generateIDsMiddlewaresNetworks(n int,
	log zerolog.Logger,
	csize int,
	tops []topology.Topology,
	states []*protocol.ReadOnlyState,
	dryrun bool) ([]*flow.Identity, []*libp2p.Middleware, []*libp2p.Network, error) {

	ids, mws, err := generateIDsAndMiddlewares(n, log)
	if err != nil {
		return nil, nil, nil, err
	}

	networks, err := generateNetworks(log, ids, mws, csize, tops, states, dryrun)
	if err != nil {
		return nil, nil, nil, err
	}

	return ids, mws, networks, nil
}

// generateEngines generates MeshEngines for the given networks
func generateEngines(t *testing.T, nets []*libp2p.Network) []*MeshEngine {
	count := len(nets)
	engs := make([]*MeshEngine, count)
	for i, n := range nets {
		eng := NewMeshEngine(t, n, 100, engine.TestNetwork)
		engs[i] = eng
	}
	return engs
}

// generateStateSnapshot generates a state and snapshot mock to return the given ids
func generateStateSnapshot(ids flow.IdentityList) *protocol.ReadOnlyState {
	state := new(protocol.ReadOnlyState)
	snapshot := new(protocol.Snapshot)
	state.On("Final").Return(snapshot, nil)
	snapshot.On("Identities", mock2.Anything).Return(ids, nil)
	return state
}
