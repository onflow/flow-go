package internal

import (
	"github.com/libp2p/go-libp2p/core/peer"

	"github.com/onflow/flow-go/module/mempool/stdmap"
)

type SubscriptionCache struct {
	c            *stdmap.Backend
	currentCycle uint64
}

func (s *SubscriptionCache) GetSubscribedTopics(id peer.ID) []string {

}

func (s *SubscriptionCache) MoveToNextUpdateCycle() {
	s.currentCycle++
}

func (s *SubscriptionCache) AddTopicForPeer(id peer.ID, topic string) bool {

}
