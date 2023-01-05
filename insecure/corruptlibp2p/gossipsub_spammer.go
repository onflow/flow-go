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
	"github.com/onflow/flow-go/network/p2p"
	p2ptest "github.com/onflow/flow-go/network/p2p/test"
)

// GossipSubRouterSpammer is a wrapper around the GossipSubRouter that allows us to
// spam the victim with junk control messages.
type GossipSubRouterSpammer struct {
	router      *atomicRouter
	SpammerNode p2p.LibP2PNode
}

// NewGossipSubRouterSpammer is the main method tests call for spamming attacks.
func NewGossipSubRouterSpammer(t *testing.T, sporkId flow.Identifier, role flow.Role) *GossipSubRouterSpammer {
	spammerNode, router := createSpammerNode(t, sporkId, role)
	return &GossipSubRouterSpammer{
		router:      router,
		SpammerNode: spammerNode,
	}
}

// SpamIHave spams the victim with junk iHave messages.
// ctlMessages is the list of spam messages to send to the victim node.
func (s *GossipSubRouterSpammer) SpamIHave(t *testing.T, victim p2p.LibP2PNode, ctlMessages []pb.ControlMessage) {
	for _, ctlMessage := range ctlMessages {
		require.True(t, s.router.Get().SendControl(victim.Host().ID(), &ctlMessage))
	}
}

// GenerateIHaveCtlMessages generates IHAVE control messages before they are sent so the test can prepare
// to expect receiving them before they are sent by the spammer.
func (s *GossipSubRouterSpammer) GenerateIHaveCtlMessages(t *testing.T, msgCount, msgSize int) []pb.ControlMessage {
	var iHaveCtlMsgs []pb.ControlMessage
	for i := 0; i < msgCount; i++ {
		iHaveCtlMsg := GossipSubCtrlFixture(WithIHave(msgCount, msgSize))

		iHaves := iHaveCtlMsg.GetIhave()
		require.Equal(t, msgCount, len(iHaves))
		iHaveCtlMsgs = append(iHaveCtlMsgs, *iHaveCtlMsg)
	}
	return iHaveCtlMsgs
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

func createSpammerNode(t *testing.T, sporkId flow.Identifier, role flow.Role) (p2p.LibP2PNode, *atomicRouter) {
	router := newAtomicRouter()
	spammerNode, _ := p2ptest.NodeFixture(
		t,
		sporkId,
		t.Name(),
		p2ptest.WithRole(role),
		internal.WithCorruptGossipSub(CorruptGossipSubFactory(func(r *corrupt.GossipSubRouter) {
			require.NotNil(t, r)
			router.set(r)
		}),
			CorruptGossipSubConfigFactoryWithInspector(func(id peer.ID, rpc *corrupt.RPC) error {
				// here we can inspect the incoming RPC message to the spammer node
				return nil
			})),
	)
	return spammerNode, router
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
