package test

import (
	"context"
	"fmt"
	"reflect"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/peer"
	pc "github.com/libp2p/go-libp2p-core/protocol"
	"github.com/libp2p/go-libp2p-core/routing"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/crypto"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/model/libp2p/message"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/id"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/module/mock"
	"github.com/onflow/flow-go/module/observable"
	"github.com/onflow/flow-go/module/util"
	"github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/network/codec/cbor"
	"github.com/onflow/flow-go/network/p2p"
	"github.com/onflow/flow-go/network/p2p/unicast"
	"github.com/onflow/flow-go/network/topology"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/utils/unittest"
)

var sporkID = unittest.IdentifierFixture()

type PeerTag struct {
	peer peer.ID
	tag  string
}

type TagWatchingConnManager struct {
	*p2p.ConnManager
	observers map[observable.Observer]struct{}
	obsLock   sync.RWMutex
}

func (cwcm *TagWatchingConnManager) Subscribe(observer observable.Observer) {
	cwcm.obsLock.Lock()
	defer cwcm.obsLock.Unlock()
	var void struct{}
	cwcm.observers[observer] = void
}

func (cwcm *TagWatchingConnManager) Unsubscribe(observer observable.Observer) {
	cwcm.obsLock.Lock()
	defer cwcm.obsLock.Unlock()
	delete(cwcm.observers, observer)
}

func (cwcm *TagWatchingConnManager) Protect(id peer.ID, tag string) {
	cwcm.obsLock.RLock()
	defer cwcm.obsLock.RUnlock()
	cwcm.ConnManager.Protect(id, tag)
	for obs := range cwcm.observers {
		go obs.OnNext(PeerTag{peer: id, tag: tag})
	}
}

func (cwcm *TagWatchingConnManager) Unprotect(id peer.ID, tag string) bool {
	cwcm.obsLock.RLock()
	defer cwcm.obsLock.RUnlock()
	res := cwcm.ConnManager.Unprotect(id, tag)
	for obs := range cwcm.observers {
		go obs.OnNext(PeerTag{peer: id, tag: tag})
	}
	return res
}

func NewTagWatchingConnManager(log zerolog.Logger, idProvider id.IdentityProvider, metrics module.NetworkMetrics) *TagWatchingConnManager {
	cm := p2p.NewConnManager(log, metrics)
	return &TagWatchingConnManager{
		ConnManager: cm,
		observers:   make(map[observable.Observer]struct{}),
		obsLock:     sync.RWMutex{},
	}
}

// GenerateIDs is a test helper that generate flow identities with a valid port and libp2p nodes.
func GenerateIDs(
	t *testing.T,
	logger zerolog.Logger,
	n int,
	opts ...func(*optsConfig),
) (flow.IdentityList, []*p2p.Node, []observable.Observable) {
	libP2PNodes := make([]*p2p.Node, n)
	tagObservables := make([]observable.Observable, n)

	o := &optsConfig{}
	for _, opt := range opts {
		opt(o)
	}

	identities := unittest.IdentityListFixture(n, o.idOpts...)

	idProvider := id.NewFixedIdentityProvider(identities)

	// generates keys and address for the node
	for i, id := range identities {
		// generate key
		key, err := generateNetworkingKey(id.NodeID)
		require.NoError(t, err)

		var opts []nodeBuilderOption

		opts = append(opts, withDHT(o.dhtPrefix, o.dhtOpts...))

		libP2PNodes[i], tagObservables[i] = generateLibP2PNode(t, logger, *id, key, o.connectionGating, idProvider, opts...)

		_, port, err := libP2PNodes[i].GetIPPort()
		require.NoError(t, err)

		identities[i].Address = fmt.Sprintf("0.0.0.0:%s", port)
		identities[i].NetworkPubKey = key.PublicKey()
	}

	return identities, libP2PNodes, tagObservables
}

// GenerateMiddlewares creates and initializes middleware instances for all the identities
func GenerateMiddlewares(t *testing.T, logger zerolog.Logger, identities flow.IdentityList, libP2PNodes []*p2p.Node, opts ...func(*optsConfig)) ([]network.Middleware, []*UpdatableIDProvider) {
	metrics := metrics.NewNoopCollector()
	mws := make([]network.Middleware, len(identities))
	idProviders := make([]*UpdatableIDProvider, len(identities))

	o := &optsConfig{}
	for _, opt := range opts {
		opt(o)
	}

	total := len(identities)
	for i := 0; i < total; i++ {
		// casts libP2PNode instance to a local variable to avoid closure
		node := libP2PNodes[i]
		nodeId := identities[i].NodeID

		// libp2p node factory for this instance of middleware
		factory := func(ctx context.Context) (*p2p.Node, error) {
			return node, nil
		}

		idProviders[i] = NewUpdatableIDProvider(identities)

		peerManagerFactory := p2p.PeerManagerFactory(o.peerManagerOpts)

		// creating middleware of nodes
		mws[i] = p2p.NewMiddleware(logger,
			factory,
			nodeId,
			metrics,
			sporkID,
			p2p.DefaultUnicastTimeout,
			p2p.NewIdentityProviderIDTranslator(idProviders[i]),
			p2p.WithPeerManager(peerManagerFactory),
		)
	}
	return mws, idProviders
}

// GenerateNetworks generates the network for the given middlewares
func GenerateNetworks(
	ctx context.Context,
	t *testing.T,
	log zerolog.Logger,
	ids flow.IdentityList,
	mws []network.Middleware,
	csize int,
	tops []network.Topology,
	sms []network.SubscriptionManager,
) []network.Network {
	count := len(ids)
	nets := make([]network.Network, 0)
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
		tops = GenerateTopologies(t, state, ids, log)
	}

	for i := 0; i < count; i++ {

		// creates and mocks me
		me := &mock.Local{}
		me.On("NodeID").Return(ids[i].NodeID)
		me.On("NotMeFilter").Return(filter.Not(filter.HasNodeID(me.NodeID())))
		me.On("Address").Return(ids[i].Address)

		// create the network
		net, err := p2p.NewNetwork(
			log,
			cbor.NewCodec(),
			me,
			func() (network.Middleware, error) { return mws[i], nil },
			csize,
			tops[i],
			sms[i],
			metrics,
			id.NewFixedIdentityProvider(ids),
		)
		require.NoError(t, err)

		nets = append(nets, net)
	}

	netCtx, errChan := irrecoverable.WithSignaler(ctx)

	go func() {
		select {
		case err := <-errChan:
			t.Error("networks encountered fatal error", err)
		case <-ctx.Done():
			return
		}
	}()

	for _, net := range nets {
		net.Start(netCtx)
		<-net.Ready()
	}

	return nets
}

// GenerateIDsAndMiddlewares returns nodeIDs, middlewares, and observables which can be subscirbed to in order to witness protect events from pubsub
func GenerateIDsAndMiddlewares(t *testing.T,
	n int,
	logger zerolog.Logger,
	opts ...func(*optsConfig),
) (flow.IdentityList, []network.Middleware, []observable.Observable, []*UpdatableIDProvider) {

	ids, libP2PNodes, protectObservables := GenerateIDs(t, logger, n, opts...)
	mws, providers := GenerateMiddlewares(t, logger, ids, libP2PNodes, opts...)
	return ids, mws, protectObservables, providers
}

type optsConfig struct {
	idOpts           []func(*flow.Identity)
	dhtPrefix        string
	dhtOpts          []dht.Option
	peerManagerOpts  []p2p.Option
	connectionGating bool
}

func WithIdentityOpts(idOpts ...func(*flow.Identity)) func(*optsConfig) {
	return func(o *optsConfig) {
		o.idOpts = idOpts
	}
}

func WithDHT(prefix string, dhtOpts ...dht.Option) func(*optsConfig) {
	return func(o *optsConfig) {
		o.dhtPrefix = prefix
		o.dhtOpts = dhtOpts
	}
}

func WithPeerManagerOpts(peerManagerOpts ...p2p.Option) func(*optsConfig) {
	return func(o *optsConfig) {
		o.peerManagerOpts = peerManagerOpts
	}
}

func GenerateIDsMiddlewaresNetworks(
	ctx context.Context,
	t *testing.T,
	n int,
	log zerolog.Logger,
	csize int,
	tops []network.Topology,
	opts ...func(*optsConfig),
) (flow.IdentityList, []network.Middleware, []network.Network, []observable.Observable) {
	ids, mws, observables, _ := GenerateIDsAndMiddlewares(t, n, log, opts...)
	sms := GenerateSubscriptionManagers(t, mws)
	networks := GenerateNetworks(ctx, t, log, ids, mws, csize, tops, sms)
	return ids, mws, networks, observables
}

// GenerateEngines generates MeshEngines for the given networks
func GenerateEngines(t *testing.T, nets []network.Network) []*MeshEngine {
	count := len(nets)
	engs := make([]*MeshEngine, count)
	for i, n := range nets {
		eng := NewMeshEngine(t, n, 100, engine.TestNetwork)
		engs[i] = eng
	}
	return engs
}

type nodeBuilderOption func(p2p.NodeBuilder)

func withDHT(prefix string, dhtOpts ...dht.Option) nodeBuilderOption {
	return func(nb p2p.NodeBuilder) {
		nb.SetRoutingSystem(func(c context.Context, h host.Host) (routing.Routing, error) {
			return p2p.NewDHT(c, h, pc.ID(unicast.FlowDHTProtocolIDPrefix+prefix), dhtOpts...)
		})
	}
}

// generateLibP2PNode generates a `LibP2PNode` on localhost using a port assigned by the OS
func generateLibP2PNode(
	t *testing.T,
	logger zerolog.Logger,
	id flow.Identity,
	key crypto.PrivateKey,
	connGating bool,
	idProvider id.IdentityProvider,
	opts ...nodeBuilderOption,
) (*p2p.Node, observable.Observable) {

	noopMetrics := metrics.NewNoopCollector()

	ctx := context.TODO()

	// Inject some logic to be able to observe connections of this node
	connManager := NewTagWatchingConnManager(logger, idProvider, noopMetrics)

	builder := p2p.NewNodeBuilder(logger, "0.0.0.0:0", key, sporkID).
		SetConnectionManager(connManager).
		SetPubSub(pubsub.NewGossipSub)

	for _, opt := range opts {
		opt(builder)
	}

	libP2PNode, err := builder.Build(ctx)
	require.NoError(t, err)

	return libP2PNode, connManager
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
func GenerateTopologies(t *testing.T, state protocol.State, identities flow.IdentityList, logger zerolog.Logger) []network.Topology {
	tops := make([]network.Topology, 0)
	for _, id := range identities {
		var top network.Topology
		var err error

		top, err = topology.NewTopicBasedTopology(id.NodeID, logger, state)
		require.NoError(t, err)

		tops = append(tops, top)
	}
	return tops
}

// GenerateSubscriptionManagers creates and returns a ChannelSubscriptionManager for each middleware object.
func GenerateSubscriptionManagers(t *testing.T, mws []network.Middleware) []network.SubscriptionManager {
	require.NotEmpty(t, mws)

	sms := make([]network.SubscriptionManager, len(mws))
	for i, mw := range mws {
		sms[i] = p2p.NewChannelSubscriptionManager(mw)
	}
	return sms
}

// stopNetworks stops network instances in parallel and fails the test if they could not be stopped within the
// duration.
func stopNetworks(t *testing.T, nets []network.Network, duration time.Duration) {

	// casts nets instances into ReadyDoneAware components
	comps := make([]module.ReadyDoneAware, 0, len(nets))
	for _, net := range nets {
		comps = append(comps, net)
	}

	unittest.RequireCloseBefore(t, util.AllDone(comps...), duration,
		"could not stop the networks")
}

// networkPayloadFixture creates a blob of random bytes with the given size (in bytes) and returns it.
// The primary goal of utilizing this helper function is to apply stress tests on the network layer by
// sending large messages to transmit.
func networkPayloadFixture(t *testing.T, size uint) []byte {
	// reserves 1000 bytes for the message headers, encoding overhead, and libp2p message overhead.
	overhead := 1000
	require.Greater(t, int(size), overhead, "could not generate message below size threshold")
	emptyEvent := &message.TestMessage{
		Text: "",
	}

	// encodes the message
	codec := cbor.NewCodec()
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
