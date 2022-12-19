package corruptlibp2p

import (
	pb "github.com/libp2p/go-libp2p-pubsub/pb"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/require"
	corrupt "github.com/yhassanzadeh13/go-libp2p-pubsub"

	"github.com/onflow/flow-go/insecure/internal"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/network/p2p"
	p2ptest "github.com/onflow/flow-go/network/p2p/test"

	"sync"
	"testing"
)

type ControlMessage int

// GossipSubRouterSpammer is a wrapper around the GossipSubRouter that allows us to
// spam the victim with junk control messages.
type GossipSubRouterSpammer struct {
	router *corrupt.GossipSubRouter
}

func NewGossipSubRouterSpammer(router *corrupt.GossipSubRouter) *GossipSubRouterSpammer {
	return &GossipSubRouterSpammer{
		router: router,
	}
}

// SpamIHave spams the victim with junk iHave messages.
// msgCount is the number of iHave messages to send.
// msgSize is the number of messageIDs to include in each iHave message.
func (s *GossipSubRouterSpammer) SpamIHave(t *testing.T, victim peer.ID, ctlMessages []pb.ControlMessage) {
	for _, ctlMessage := range ctlMessages {
		require.True(t, s.router.SendControl(victim, &ctlMessage))
	}
}

// GenerateIHaveCtlMessages generates IHAVE control messages before they are sent so the test can prepare
// to receive them before they are sent by the spammer.
func (s *GossipSubRouterSpammer) GenerateIHaveCtlMessages(t *testing.T, msgCount, msgSize int) []pb.ControlMessage {
	//var ctlMessageMap = make(map[string]pb.ControlMessage)
	var iHaveCtlMsgs []pb.ControlMessage
	for i := 0; i < msgCount; i++ {
		iHaveCtlMsg := GossipSubCtrlFixture(WithIHave(msgCount, msgSize))

		iHaves := iHaveCtlMsg.GetIhave()
		require.Equal(t, msgCount, len(iHaves))
		iHaveCtlMsgs = append(iHaveCtlMsgs, *iHaveCtlMsg)
	}
	return iHaveCtlMsgs
}

func GetSpammerNode(t *testing.T, sporkId flow.Identifier) (p2p.LibP2PNode, flow.Identity, *atomicRouter) {
	router := newAtomicRouter()
	spammerNode, spammerId := p2ptest.NodeFixture(
		t,
		sporkId,
		t.Name(),
		p2ptest.WithRole(flow.RoleConsensus),
		internal.WithCorruptGossipSub(CorruptGossipSubFactory(func(r *corrupt.GossipSubRouter) {
			require.NotNil(t, r)
			router.set(r)
		}),
			CorruptGossipSubConfigFactoryWithInspector(func(id peer.ID, rpc *corrupt.RPC) error {
				// here we can inspect the incoming RPC message to the spammer node
				return nil
			})),
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

// SetRouter sets the router if it has never been set.
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
