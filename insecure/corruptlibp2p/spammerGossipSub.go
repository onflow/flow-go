package corruptlibp2p

import (
	"github.com/libp2p/go-libp2p/core/peer"
	pubsub "github.com/yhassanzadeh13/go-libp2p-pubsub"
)

type ControlMessage int

// SpammerGossipSub is a wrapper around the GossipSubRouter that allows us to
// spam the victim with junk control messages.
type SpammerGossipSub struct {
	router *pubsub.GossipSubRouter
}

func NewSpammerGossipSubRouter(router *pubsub.GossipSubRouter) *SpammerGossipSub {
	return &SpammerGossipSub{
		router: router,
	}
}

// SpamIHave spams the victim with junk iHave messages.
// msgCount is the number of iHave messages to send.
// msgSize is the number of messageIDs to include in each iHave message.
func (s *SpammerGossipSub) SpamIHave(victim peer.ID, msgCount int, msgSize int) {
	for i := 0; i < msgCount; i++ {
		ctlIHave := GossipSubCtrlFixture(WithIHave(msgCount, msgSize))
		s.router.SendControl(victim, ctlIHave)
	}
}

// TODO: SpamIWant, SpamGraft, SpamPrune.
