package corruptlibp2p

import (
	"sync"
	"testing"
	"time"

	pb "github.com/libp2p/go-libp2p-pubsub/pb"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/require"

	corrupt "github.com/yhassanzadeh13/go-libp2p-pubsub"

	"github.com/onflow/flow-go/insecure/internal"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/network/p2p"
	p2ptest "github.com/onflow/flow-go/network/p2p/test"
)

// GossipSubRouterSpammer is a wrapper around the GossipSubRouter that allows us to
// spam the victim with junk control messages.
type GossipSubRouterSpammer struct {
	router      *atomicRouter
	SpammerNode p2p.LibP2PNode
	SpammerId   flow.Identity
}

// NewGossipSubRouterSpammer creates a new GossipSubRouterSpammer.
// Args:
// - t: the test object.
// - sporkId: the spork node's ID.
// - role: the role of the spork node.
// - provider: the identity provider.
// Returns:
// - the GossipSubRouterSpammer.
func NewGossipSubRouterSpammer(t *testing.T, sporkId flow.Identifier, role flow.Role, provider module.IdentityProvider) *GossipSubRouterSpammer {
	return NewGossipSubRouterSpammerWithRpcInspector(t, sporkId, role, provider, func(id peer.ID, rpc *corrupt.RPC) error {
		return nil // no-op
	})
}

// NewGossipSubRouterSpammerWithRpcInspector creates a new GossipSubRouterSpammer with a custom RPC inspector.
// The RPC inspector is called before each incoming RPC is processed by the router.
// If the inspector returns an error, the RPC is dropped.
// Args:
// - t: the test object.
// - sporkId: the spork node's ID.
// - role: the role of the spork node.
// - provider: the identity provider.
// - inspector: the RPC inspector.
// Returns:
// - the GossipSubRouterSpammer.
func NewGossipSubRouterSpammerWithRpcInspector(t *testing.T, sporkId flow.Identifier, role flow.Role, provider module.IdentityProvider, inspector func(id peer.ID, rpc *corrupt.RPC) error) *GossipSubRouterSpammer {
	spammerNode, spammerId, router := newSpammerNodeWithRpcInspector(t, sporkId, role, provider, inspector)
	return &GossipSubRouterSpammer{
		router:      router,
		SpammerNode: spammerNode,
		SpammerId:   spammerId,
	}
}

// SpamControlMessage spams the victim with junk control messages.
// ctlMessages is the list of spam messages to send to the victim node.
func (s *GossipSubRouterSpammer) SpamControlMessage(t *testing.T, victim p2p.LibP2PNode, ctlMessages []pb.ControlMessage, msgs ...*pb.Message) {
	for _, ctlMessage := range ctlMessages {
		require.True(t, s.router.Get().SendControl(victim.ID(), &ctlMessage, msgs...))
	}
}

// GenerateCtlMessages generates control messages before they are sent so the test can prepare
// to expect receiving them before they are sent by the spammer.
func (s *GossipSubRouterSpammer) GenerateCtlMessages(msgCount int, opts ...GossipSubCtrlOption) []pb.ControlMessage {
	var ctlMgs []pb.ControlMessage
	for i := 0; i < msgCount; i++ {
		ctlMsg := GossipSubCtrlFixture(opts...)
		ctlMgs = append(ctlMgs, *ctlMsg)
	}
	return ctlMgs
}

// Start starts the spammer and waits until it is fully initialized before returning.
func (s *GossipSubRouterSpammer) Start(t *testing.T) {
	require.Eventuallyf(t, func() bool {
		// ensuring the spammer router has been initialized.
		// this is needed because the router is initialized asynchronously.
		return s.router.Get() != nil
	}, 1*time.Second, 100*time.Millisecond, "spammer router not set")
	s.router.set(s.router.Get())
}

// newSpammerNodeWithRpcInspector creates a new spammer node, which is capable of sending spam control and actual messages to other nodes.
// It also creates a new atomic router that allows us to set the router to a new instance of the corrupt router.
// Args:
// - sporkId: the spork id of the spammer node.
// - role: the role of the spammer node.
// - provider: the identity provider of the spammer node.
// - inspector: the inspector function that is called when a message is received by the spammer node.
// Returns:
// - p2p.LibP2PNode: the spammer node.
// - flow.Identity: the identity of the spammer node.
// - *atomicRouter: the atomic router that allows us to set the router to a new instance of the corrupt router.
func newSpammerNodeWithRpcInspector(
	t *testing.T,
	sporkId flow.Identifier,
	role flow.Role,
	provider module.IdentityProvider,
	inspector func(id peer.ID, rpc *corrupt.RPC) error) (p2p.LibP2PNode, flow.Identity, *atomicRouter) {
	router := newAtomicRouter()
	var opts []p2ptest.NodeFixtureParameterOption
	opts = append(opts, p2ptest.WithRole(role),
		internal.WithCorruptGossipSub(CorruptGossipSubFactory(func(r *corrupt.GossipSubRouter) {
			require.NotNil(t, r)
			router.set(r)
		}),
			CorruptGossipSubConfigFactoryWithInspector(inspector)))
	spammerNode, spammerId := p2ptest.NodeFixture(
		t,
		sporkId,
		t.Name(),
		provider,
		opts...,
	)
	return spammerNode, spammerId, router
}

// atomicRouter is a wrapper around the corrupt.GossipSubRouter that allows atomic access to the router.
// This is done to avoid race conditions when accessing the router from multiple goroutines.
type atomicRouter struct {
	mu     sync.Mutex
	router *corrupt.GossipSubRouter
}

func newAtomicRouter() *atomicRouter {
	return &atomicRouter{
		mu: sync.Mutex{},
	}
}

// SetRouter sets the router if it has never been set. Returns true if the router was set, false otherwise.
func (a *atomicRouter) set(router *corrupt.GossipSubRouter) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.router == nil {
		a.router = router
		return true
	}
	return false
}

// Get returns the router.
func (a *atomicRouter) Get() *corrupt.GossipSubRouter {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.router
}

// TODO: SpamIWant, SpamGraft, SpamPrune.
