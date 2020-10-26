package test

import (
	"fmt"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/phayes/freeport"
	"github.com/rs/zerolog"
	mock3 "github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/module/mock"
	"github.com/onflow/flow-go/network/codec/json"
	"github.com/onflow/flow-go/network/gossip/libp2p"
	"github.com/onflow/flow-go/network/gossip/libp2p/channel"
	mock2 "github.com/onflow/flow-go/network/gossip/libp2p/mock"
	"github.com/onflow/flow-go/network/gossip/libp2p/topology"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/utils/unittest"
)

var rootBlockID = unittest.IdentifierFixture().String()

const (
	DryRunNetwork = "dry-run-network" // does not run network
	RunNetwork    = "run-network"     // runs network
)

// GenerateIDs generate flow Identities with a valid port and networking key
func GenerateIDs(t *testing.T, n int, runningMode string, opts ...func(*flow.Identity)) (flow.IdentityList,
	[]crypto.PrivateKey) {
	privateKeys := make([]crypto.PrivateKey, n)
	var freePorts []int
	var err error

	if !strings.EqualFold(runningMode, DryRunNetwork) {
		// get free ports
		freePorts, err = freeport.GetFreePorts(n)
		require.NoError(t, err)
	}

	identities := unittest.IdentityListFixture(n, opts...)

	// generates keys and address for the node
	for i, id := range identities {
		// generate key
		key, err := GenerateNetworkingKey(id.NodeID)
		require.NoError(t, err)
		privateKeys[i] = key
		port := 0

		if !strings.EqualFold(runningMode, DryRunNetwork) {
			port = freePorts[i]
		}

		identities[i].Address = fmt.Sprintf("0.0.0.0:%d", port)
		identities[i].NetworkPubKey = key.PublicKey()
	}
	return identities, privateKeys
}

// GenerateMiddlewares creates and initializes middleware instances for all the identities
func GenerateMiddlewares(t *testing.T, log zerolog.Logger, identities flow.IdentityList, keys []crypto.PrivateKey) []*libp2p.Middleware {
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
		require.NoError(t, err)
		mws[i] = mw
	}
	return mws
}

// GenerateNetworks generates the network for the given middlewares
func GenerateNetworks(t *testing.T,
	log zerolog.Logger,
	ids flow.IdentityList,
	mws []*libp2p.Middleware,
	csize int,
	tops []topology.Topology,
	sms []channel.SubscriptionManager,
	runningMode string) []*libp2p.Network {
	count := len(ids)
	nets := make([]*libp2p.Network, 0)
	metrics := metrics.NewNoopCollector()

	if tops == nil {
		// creates default topology
		//
		// mocks state for collector nodes topology
		// considers only a single cluster as higher cluster numbers are tested
		// in collectionTopology_test
		state := topology.CreateMockStateForCollectionNodes(t,
			ids.Filter(filter.HasRole(flow.RoleCollection)), 1)
		// creates topology instances for the nodes based on their roles
		tops = GenerateTopologies(t, state, ids)
	}

	for i := 0; i < count; i++ {

		// creates and mocks me
		me := &mock.Local{}
		me.On("NodeID").Return(ids[i].NodeID)
		me.On("NotMeFilter").Return(filter.Not(filter.HasNodeID(me.NodeID())))
		me.On("Address").Return(ids[i].Address)

		// create the network
		net, err := libp2p.NewNetwork(log, json.NewCodec(), ids, me, mws[i], csize, tops[i], sms[i], metrics)
		require.NoError(t, err)

		nets = append(nets, net)
	}

	// if dryrun then don't actually start the network
	if !strings.EqualFold(runningMode, DryRunNetwork) {
		for _, net := range nets {
			<-net.Ready()
		}
	}
	return nets
}

func GenerateIDsAndMiddlewares(t *testing.T,
	n int,
	runningMode string,
	log zerolog.Logger) (flow.IdentityList,
	[]*libp2p.Middleware) {
	ids, keys := GenerateIDs(t, n, runningMode)
	mws := GenerateMiddlewares(t, log, ids, keys)
	return ids, mws
}

func GenerateIDsMiddlewaresNetworks(t *testing.T,
	n int,
	log zerolog.Logger,
	csize int,
	tops []topology.Topology,
	runninMode string) (flow.IdentityList, []*libp2p.Middleware, []*libp2p.Network) {
	ids, mws := GenerateIDsAndMiddlewares(t, n, runninMode, log)
	sms := GenerateSubscriptionManagers(t, mws)
	networks := GenerateNetworks(t, log, ids, mws, csize, tops, sms, runninMode)
	return ids, mws, networks
}

// GenerateEngines generates MeshEngines for the given networks
func GenerateEngines(t *testing.T, nets []*libp2p.Network) []*MeshEngine {
	count := len(nets)
	engs := make([]*MeshEngine, count)
	for i, n := range nets {
		eng := NewMeshEngine(t, n, 100, engine.TestNetwork)
		engs[i] = eng
	}
	return engs
}

// OptionalSleep introduces a sleep to allow nodes to heartbeat and discover each other (only needed when using PubSub)
func optionalSleep(send ConduitSendWrapperFunc) {
	sendFuncName := runtime.FuncForPC(reflect.ValueOf(send).Pointer()).Name()
	if strings.Contains(sendFuncName, "Multicast") || strings.Contains(sendFuncName, "Publish") {
		time.Sleep(2 * time.Second)
	}
}

// GenerateNetworkingKey generates a Flow ECDSA key using the given seed
func GenerateNetworkingKey(s flow.Identifier) (crypto.PrivateKey, error) {
	seed := make([]byte, crypto.KeyGenSeedMinLenECDSASecp256k1)
	copy(seed, s[:])
	return crypto.GeneratePrivateKey(crypto.ECDSASecp256k1, seed)
}

// CreateTopologies is a test helper on receiving an identity list, creates a topology per identity
// and returns the slice of topologies.
func GenerateTopologies(t *testing.T, state protocol.State, identities flow.IdentityList) []topology.Topology {
	tops := make([]topology.Topology, 0)
	for _, id := range identities {
		var top topology.Topology
		var err error

		top, err = topology.NewTopicBasedTopology(id.NodeID, state)

		require.NoError(t, err)
		tops = append(tops, top)
	}
	return tops
}

// GenerateSubscriptionManagers creates and returns a ChannelSubscriptionManager for each middleware object.
func GenerateSubscriptionManagers(t *testing.T, mws []*libp2p.Middleware) []channel.SubscriptionManager {
	require.NotEmpty(t, mws)

	sms := make([]channel.SubscriptionManager, len(mws))
	for i, mw := range mws {
		sms[i] = libp2p.NewSubscriptionManager(mw)
	}
	return sms
}

// MockSubscriptionManager returns a list of mocked subscription manages for the input
// identities. It only mocks the GetChannelIDs method of the subscription manager. Other methods
// return an error, as they are not supposed to be invoked.
func MockSubscriptionManager(t *testing.T, ids flow.IdentityList) []channel.SubscriptionManager {
	require.NotEmpty(t, ids)

	sms := make([]channel.SubscriptionManager, len(ids))
	for i, id := range ids {
		sm := &mock2.SubscriptionManager{}
		err := fmt.Errorf("this method should not be called on mock subscription manager")
		sm.On("Register", mock3.Anything, mock3.Anything).Return(err)
		sm.On("Unregister", mock3.Anything).Return(err)
		sm.On("GetEngine", mock3.Anything).Return(err)
		sm.On("GetChannelIDs").Return(engine.ChannelIDsByRole(id.Role))
		sms[i] = sm
	}

	return sms
}
