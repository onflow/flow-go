package test

import (
	"fmt"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"

	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
	message2 "github.com/onflow/flow-go/model/libp2p/message"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/lifecycle"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/module/mock"
	"github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/network/codec/json"
	"github.com/onflow/flow-go/network/p2p"
	"github.com/onflow/flow-go/network/topology"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/utils/unittest"
)

var rootBlockID = unittest.IdentifierFixture().String()

const DryRun = true

// GenerateIDs is a test helper that generate flow identities with a valid port and libp2p nodes.
// If `dryRunMode` is set to true, it returns an empty slice instead of libp2p nodes, assuming that slice is never going
// to get used.
func GenerateIDs(t *testing.T, logger zerolog.Logger, n int, dryRunMode bool, opts ...func(*flow.Identity)) (flow.IdentityList, []*p2p.Node) {
	libP2PNodes := make([]*p2p.Node, n)

	identities := unittest.IdentityListFixture(n, opts...)

	// generates keys and address for the node
	for i, id := range identities {
		// generate key
		key, err := generateNetworkingKey(id.NodeID)
		require.NoError(t, err)
		port := "0"

		if !dryRunMode {
			libP2PNodes[i] = generateLibP2PNode(t, logger, *id, key)
			_, port, err = libP2PNodes[i].GetIPPort()
			require.NoError(t, err)
		}

		identities[i].Address = fmt.Sprintf("0.0.0.0:%s", port)
		identities[i].NetworkPubKey = key.PublicKey()
	}
	return identities, libP2PNodes
}

// GenerateMiddlewares creates and initializes middleware instances for all the identities
func GenerateMiddlewares(t *testing.T, logger zerolog.Logger, identities flow.IdentityList, libP2PNodes []*p2p.Node) []*p2p.Middleware {
	metrics := metrics.NewNoopCollector()
	mws := make([]*p2p.Middleware, len(identities))

	for i, id := range identities {
		// casts libP2PNode instance to a local variable to avoid closure
		node := libP2PNodes[i]

		// libp2p node factory for this instance of middleware
		factory := func() (*p2p.Node, error) {
			return node, nil
		}

		// creating middleware of nodes
		mws[i] = p2p.NewMiddleware(logger,
			factory,
			id.NodeID,
			metrics,
			p2p.DefaultMaxUnicastMsgSize,
			p2p.DefaultMaxPubSubMsgSize,
			rootBlockID)
	}
	return mws
}

// GenerateNetworks generates the network for the given middlewares
func GenerateNetworks(t *testing.T,
	log zerolog.Logger,
	ids flow.IdentityList,
	mws []*p2p.Middleware,
	csize int,
	tops []network.Topology,
	sms []network.SubscriptionManager,
	dryRunMode bool) []*p2p.Network {
	count := len(ids)
	nets := make([]*p2p.Network, 0)
	metrics := metrics.NewNoopCollector()

	// checks if necessary to generate topology managers
	if tops == nil {
		// nil topology managers means generating default ones

		// creates default topology
		//
		// mocks state for collector nodes topology
		// considers only a single cluster as higher cluster numbers are tested
		// in collectionTopology_test
		state, _ := topology.MockStateForCollectionNodes(t,
			ids.Filter(filter.HasRole(flow.RoleCollection)), 1)
		// creates topology instances for the nodes based on their roles
		tops = GenerateTopologies(t, state, ids, sms, log)
	}

	for i := 0; i < count; i++ {

		// creates and mocks me
		me := &mock.Local{}
		me.On("NodeID").Return(ids[i].NodeID)
		me.On("NotMeFilter").Return(filter.Not(filter.HasNodeID(me.NodeID())))
		me.On("Address").Return(ids[i].Address)

		// create the network
		net, err := p2p.NewNetwork(log, json.NewCodec(), ids, me, mws[i], csize, tops[i], sms[i], metrics)
		require.NoError(t, err)

		nets = append(nets, net)
	}

	// if dryrun then don't actually start the network
	if !dryRunMode {
		for _, net := range nets {
			<-net.Ready()
			err := net.SetIDs(ids)
			require.NoError(t, err)
		}
	}
	return nets
}

func GenerateIDsAndMiddlewares(t *testing.T,
	n int,
	dryRunMode bool,
	logger zerolog.Logger) (flow.IdentityList,
	[]*p2p.Middleware) {

	ids, libP2PNodes := GenerateIDs(t, logger, n, dryRunMode)
	mws := GenerateMiddlewares(t, logger, ids, libP2PNodes)
	return ids, mws
}

func GenerateIDsMiddlewaresNetworks(t *testing.T,
	n int,
	log zerolog.Logger,
	csize int,
	tops []network.Topology,
	dryRun bool) (flow.IdentityList, []*p2p.Middleware, []*p2p.Network) {
	ids, mws := GenerateIDsAndMiddlewares(t, n, dryRun, log)
	sms := GenerateSubscriptionManagers(t, mws)
	networks := GenerateNetworks(t, log, ids, mws, csize, tops, sms, dryRun)
	return ids, mws, networks
}

// GenerateEngines generates MeshEngines for the given networks
func GenerateEngines(t *testing.T, nets []*p2p.Network) []*MeshEngine {
	count := len(nets)
	engs := make([]*MeshEngine, count)
	for i, n := range nets {
		eng := NewMeshEngine(t, n, 100, engine.TestNetwork)
		engs[i] = eng
	}
	return engs
}

// generateLibP2PNode generates a `LibP2PNode` on localhost using a port assigned by the OS
func generateLibP2PNode(t *testing.T,
	logger zerolog.Logger,
	id flow.Identity,
	key crypto.PrivateKey) *p2p.Node {

	noopMetrics := metrics.NewNoopCollector()

	// create PubSub options for libp2p to use
	psOptions := []pubsub.Option{
		// skip message signing
		pubsub.WithMessageSigning(false),
		// skip message signature
		pubsub.WithStrictSignatureVerification(false),
		// set max message size limit for 1-k PubSub messaging
		pubsub.WithMaxMessageSize(p2p.DefaultMaxPubSubMsgSize),
	}

	libP2PNode, err := p2p.NewLibP2PNode(logger,
		id.NodeID,
		"0.0.0.0:0",
		p2p.NewConnManager(logger, noopMetrics),
		key,
		true,
		rootBlockID,
		psOptions...)

	require.NoError(t, err)

	return libP2PNode
}

// OptionalSleep introduces a sleep to allow nodes to heartbeat and discover each other (only needed when using PubSub)
func optionalSleep(send ConduitSendWrapperFunc) {
	sendFuncName := runtime.FuncForPC(reflect.ValueOf(send).Pointer()).Name()
	if strings.Contains(sendFuncName, "Multicast") || strings.Contains(sendFuncName, "Publish") {
		time.Sleep(2 * time.Second)
	}
}

// generateNetworkingKey generates a Flow ECDSA key using the given seed
func generateNetworkingKey(s flow.Identifier) (crypto.PrivateKey, error) {
	seed := make([]byte, crypto.KeyGenSeedMinLenECDSASecp256k1)
	copy(seed, s[:])
	return crypto.GeneratePrivateKey(crypto.ECDSASecp256k1, seed)
}

// CreateTopologies is a test helper on receiving an identity list, creates a topology per identity
// and returns the slice of topologies.
func GenerateTopologies(t *testing.T, state protocol.State, identities flow.IdentityList,
	subMngrs []network.SubscriptionManager, logger zerolog.Logger) []network.Topology {
	tops := make([]network.Topology, 0)
	for i, id := range identities {
		var top network.Topology
		var err error

		top, err = topology.NewTopicBasedTopology(id.NodeID, logger, state, subMngrs[i])
		require.NoError(t, err)

		tops = append(tops, top)
	}
	return tops
}

// GenerateSubscriptionManagers creates and returns a ChannelSubscriptionManager for each middleware object.
func GenerateSubscriptionManagers(t *testing.T, mws []*p2p.Middleware) []network.SubscriptionManager {
	require.NotEmpty(t, mws)

	sms := make([]network.SubscriptionManager, len(mws))
	for i, mw := range mws {
		sms[i] = p2p.NewChannelSubscriptionManager(mw)
	}
	return sms
}

// stopNetworks stops network instances in parallel and fails the test if they could not be stopped within the
// duration.
func stopNetworks(t *testing.T, nets []*p2p.Network, duration time.Duration) {
	// casts nets instances into ReadyDoneAware components
	comps := make([]module.ReadyDoneAware, 0, len(nets))
	for _, net := range nets {
		comps = append(comps, net)
	}

	unittest.RequireCloseBefore(t, lifecycle.AllDone(comps...), duration,
		"could not stop the networks")
}

// networkPayloadFixture creates a blob of random bytes with the given size (in bytes) and returns it.
// The primary goal of utilizing this helper function is to apply stress tests on the network layer by
// sending large messages to transmit.
func networkPayloadFixture(t *testing.T, size uint) []byte {
	// reserves 1000 bytes for the message headers, encoding overhead, and libp2p message overhead.
	overhead := 1000
	require.Greater(t, int(size), overhead, "could not generate message below size threshold")
	emptyEvent := &message2.TestMessage{
		Text: "",
	}

	// encodes the message
	codec := json.NewCodec()
	empty, err := codec.Encode(emptyEvent)
	require.NoError(t, err)

	// max possible payload size
	payloadSize := int(size) - overhead - len(empty)
	payload := make([]byte, payloadSize)

	// populates payload with random bytes
	for i := range payload {
		payload[i] = 'a' // a utf-8 char that translates to 1-byte when converted to a string
	}

	event := emptyEvent
	event.Text = string(payload)
	// encode event the way the network would encode it to get the size of the message
	// just to do the size check
	encodedEvent, err := codec.Encode(event)
	require.NoError(t, err)

	require.InDelta(t, len(encodedEvent), int(size), float64(overhead))

	return payload
}
