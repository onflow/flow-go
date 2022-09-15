package scoring_test

import (
	"testing"

	"github.com/libp2p/go-libp2p/core/peer"
	mockp2p "github.com/onflow/flow-go/network/p2p/mock"
	"github.com/onflow/flow-go/network/p2p/scoring"
	"github.com/onflow/flow-go/network/p2p/unittest"
	"github.com/stretchr/testify/assert"
)

func TestSubscriptionProvider(t *testing.T) {
	tp := mockp2p.NewTopicProvider(t)
	sp := scoring.NewSubscriptionProvider(tp)

	tp.On("GetTopics").Return([]string{"topic1", "topic2", "topic3"})

	peer1 := unittest.PeerIdFixture(t)
	peer2 := unittest.PeerIdFixture(t)
	peer3 := unittest.PeerIdFixture(t)

	// mock peers 1 and 2 subscribed to topic 1 (along with other random peers)
	tp.On("ListPeers", "topic1").Return(append([]peer.ID{peer1, peer2}, unittest.PeerIdsFixture(t, 10)...))
	// mock peers 2 and 3 subscribed to topic 2 (along with other random peers)
	tp.On("ListPeers", "topic2").Return(append([]peer.ID{peer2, peer3}, unittest.PeerIdsFixture(t, 10)...))
	// mock peers 1 and 3 subscribed to topic 3 (along with other random peers)
	tp.On("ListPeers", "topic3").Return(append([]peer.ID{peer1, peer3}, unittest.PeerIdsFixture(t, 10)...))

	assert.ElementsMatchf(t, []string{"topic1", "topic3"}, sp.GetSubscribedTopics(peer1), "peer 1 should be subscribed to topics 1 and 3")
	assert.ElementsMatchf(t, []string{"topic1", "topic2"}, sp.GetSubscribedTopics(peer2), "peer 2 should be subscribed to topics 1 and 2")
	assert.ElementsMatchf(t, []string{"topic2", "topic3"}, sp.GetSubscribedTopics(peer3), "peer 3 should be subscribed to topics 2 and 3")
}
