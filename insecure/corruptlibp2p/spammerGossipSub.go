package corruptlibp2p

import (
	"github.com/libp2p/go-libp2p/core/peer"

	"github.com/onflow/flow-go/insecure/internal"
)

type ControlMessage int

// GossipSubSpammer is a wrapper around the GossipSubRouter that allows us to
// spam the victim with junk control messages.
type GossipSubSpammer struct {
	router *internal.CorruptGossipSubRouter
}

func NewGossipSubSpammer(router *internal.CorruptGossipSubRouter) *GossipSubSpammer {
	return &GossipSubSpammer{
		router: router,
	}
}

// SpamIHave spams the victim with junk iHave messages.
// msgCount is the number of iHave messages to send.
// msgSize is the number of messageIDs to include in each iHave message.
func (s *GossipSubSpammer) SpamIHave(victim peer.ID, msgCount int, msgSize int) {
	for i := 0; i < msgCount; i++ {
		ctlIHave := GossipSubCtrlFixture(WithIHave(msgCount, msgSize))
		s.router.SendControl(victim, ctlIHave)
	}
}

// TODO: SpamIWant, SpamGraft, SpamPrune.
