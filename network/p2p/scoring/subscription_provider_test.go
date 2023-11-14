package scoring_test

import (
	"testing"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/assert"

	mockp2p "github.com/onflow/flow-go/network/p2p/mock"
	"github.com/onflow/flow-go/network/p2p/scoring"
	"github.com/onflow/flow-go/utils/slices"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestSubscriptionProvider_GetSubscribedTopics tests that the SubscriptionProvider returns the correct
// list of topics a peer is subscribed to.
func TestSubscriptionProvider_GetSubscribedTopics(t *testing.T) {
	tp := mockp2p.NewTopicProvider(t)
	sp := scoring.NewSubscriptionProvider(unittest.Logger(), tp)

	tp.On("GetTopics").Return([]string{"topic1", "topic2", "topic3"}).Maybe()

	peer1 := unittest.PeerIdFixture(t)
	peer2 := unittest.PeerIdFixture(t)
	peer3 := unittest.PeerIdFixture(t)

	// mock peers 1 and 2 subscribed to topic 1 (along with other random peers)
	tp.On("ListPeers", "topic1").Return(append([]peer.ID{peer1, peer2}, unittest.PeerIdFixtures(t, 10)...))
	// mock peers 2 and 3 subscribed to topic 2 (along with other random peers)
	tp.On("ListPeers", "topic2").Return(append([]peer.ID{peer2, peer3}, unittest.PeerIdFixtures(t, 10)...))
	// mock peers 1 and 3 subscribed to topic 3 (along with other random peers)
	tp.On("ListPeers", "topic3").Return(append([]peer.ID{peer1, peer3}, unittest.PeerIdFixtures(t, 10)...))

	// As the calls to the TopicProvider are asynchronous, we need to wait for the goroutines to finish.
	assert.Eventually(t, func() bool {
		return slices.AreStringSlicesEqual([]string{"topic1", "topic3"}, sp.GetSubscribedTopics(peer1))
	}, 1*time.Second, 100*time.Millisecond)

	assert.Eventually(t, func() bool {
		return slices.AreStringSlicesEqual([]string{"topic1", "topic2"}, sp.GetSubscribedTopics(peer2))
	}, 1*time.Second, 100*time.Millisecond)

	assert.Eventually(t, func() bool {
		return slices.AreStringSlicesEqual([]string{"topic2", "topic3"}, sp.GetSubscribedTopics(peer3))
	}, 1*time.Second, 100*time.Millisecond)
}
