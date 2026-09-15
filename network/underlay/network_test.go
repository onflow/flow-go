package underlay

import (
	"context"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/metrics"
	modulemock "github.com/onflow/flow-go/module/mock"
	"github.com/onflow/flow-go/network"
	netcache "github.com/onflow/flow-go/network/cache"
	"github.com/onflow/flow-go/network/channels"
	"github.com/onflow/flow-go/network/message"
	mockmsg "github.com/onflow/flow-go/network/mock"
	p2plogging "github.com/onflow/flow-go/network/p2p/logging"
	"github.com/onflow/flow-go/network/queue"
	"github.com/onflow/flow-go/network/validator"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestIsProtocolParticipant_UnknownPeer(t *testing.T) {
	idProvider := modulemock.NewIdentityProvider(t)
	remotePeerID := unittest.PeerIdFixture(t)

	idProvider.On("ByPeerID", remotePeerID).Return(nil, false).Once()

	net := &Network{
		identityProvider: idProvider,
	}

	filter := net.isProtocolParticipant()
	err := filter(remotePeerID)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown")
}

func TestIsProtocolParticipant_EjectedPeer(t *testing.T) {
	idProvider := modulemock.NewIdentityProvider(t)
	remotePeerID := unittest.PeerIdFixture(t)

	ejectedIdentity := unittest.IdentityFixture(
		unittest.WithRole(flow.RoleExecution),
		unittest.WithParticipationStatus(flow.EpochParticipationStatusEjected),
	)
	idProvider.On("ByPeerID", remotePeerID).Return(ejectedIdentity, true).Once()

	net := &Network{
		identityProvider: idProvider,
	}

	filter := net.isProtocolParticipant()
	err := filter(remotePeerID)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ejected")
}

func TestIsProtocolParticipant_ActivePeer(t *testing.T) {
	idProvider := modulemock.NewIdentityProvider(t)
	remotePeerID := unittest.PeerIdFixture(t)

	activeIdentity := unittest.IdentityFixture(
		unittest.WithRole(flow.RoleConsensus),
		unittest.WithParticipationStatus(flow.EpochParticipationStatusActive),
	)
	idProvider.On("ByPeerID", remotePeerID).Return(activeIdentity, true).Once()

	net := &Network{
		identityProvider: idProvider,
	}

	filter := net.isProtocolParticipant()
	err := filter(remotePeerID)
	require.NoError(t, err)
}

func TestGetAuthorizedIdentity_UnknownPeer(t *testing.T) {
	idProvider := modulemock.NewIdentityProvider(t)
	violationsConsumer := mockmsg.NewViolationsConsumer(t)
	remotePeerID := unittest.PeerIdFixture(t)

	idProvider.On("ByPeerID", remotePeerID).Return(nil, false).Once()
	violationsConsumer.On("OnUnauthorizedSenderError", &network.Violation{
		PeerID:   p2plogging.PeerId(remotePeerID),
		Protocol: message.ProtocolTypeUnicast,
		Err:      validator.ErrIdentityUnverified,
	}).Once()

	net := &Network{
		identityProvider:           idProvider,
		slashingViolationsConsumer: violationsConsumer,
	}

	log := zerolog.Nop()
	identity, ok := net.getAuthorizedIdentity(log, remotePeerID)
	require.False(t, ok)
	require.Nil(t, identity)
}

func TestGetAuthorizedIdentity_EjectedPeer(t *testing.T) {
	idProvider := modulemock.NewIdentityProvider(t)
	violationsConsumer := mockmsg.NewViolationsConsumer(t)
	remotePeerID := unittest.PeerIdFixture(t)

	ejectedIdentity := unittest.IdentityFixture(
		unittest.WithRole(flow.RoleExecution),
		unittest.WithParticipationStatus(flow.EpochParticipationStatusEjected),
	)
	idProvider.On("ByPeerID", remotePeerID).Return(ejectedIdentity, true).Once()
	violationsConsumer.On("OnSenderEjectedError", &network.Violation{
		OriginID: ejectedIdentity.NodeID,
		Identity: ejectedIdentity,
		PeerID:   p2plogging.PeerId(remotePeerID),
		Protocol: message.ProtocolTypeUnicast,
		Err:      validator.ErrSenderEjected,
	}).Once()

	net := &Network{
		identityProvider:           idProvider,
		slashingViolationsConsumer: violationsConsumer,
	}

	log := zerolog.Nop()
	identity, ok := net.getAuthorizedIdentity(log, remotePeerID)
	require.False(t, ok)
	require.Nil(t, identity)
}

func TestGetAuthorizedIdentity_ActivePeer(t *testing.T) {
	idProvider := modulemock.NewIdentityProvider(t)
	violationsConsumer := mockmsg.NewViolationsConsumer(t)
	remotePeerID := unittest.PeerIdFixture(t)

	activeIdentity := unittest.IdentityFixture(
		unittest.WithRole(flow.RoleConsensus),
		unittest.WithParticipationStatus(flow.EpochParticipationStatusActive),
	)
	idProvider.On("ByPeerID", remotePeerID).Return(activeIdentity, true).Once()

	net := &Network{
		identityProvider:           idProvider,
		slashingViolationsConsumer: violationsConsumer,
	}

	log := zerolog.Nop()
	identity, ok := net.getAuthorizedIdentity(log, remotePeerID)
	require.True(t, ok)
	require.Equal(t, activeIdentity, identity)
}

// TestProcessNetworkMessage_QueueFullDropRollsBackDedupCache verifies that when a message is
// dropped because the inbound queue is full, its dedup cache entry is rolled back, so a later
// retransmission of the same payload is accepted. The dedup event ID is sender-independent
// (a hash of channel and payload), so a stale entry would drop identical payloads from any sender.
func TestProcessNetworkMessage_QueueFullDropRollsBackDedupCache(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel) // unblocks the queue's shutdown goroutine

	metricsCollector := modulemock.NewNetworkCoreMetrics(t)

	// Only the fields touched by processNetworkMessage (receiveCache, queue, metrics, logger)
	// are needed; this mirrors a Network built with MessageQueueSize=1.
	n := &Network{
		logger:       zerolog.Nop(),
		metrics:      metricsCollector,
		receiveCache: netcache.NewReceiveCache(1000),
		queue:        queue.NewMessageQueue(ctx, queue.GetEventPriority, metrics.NewNoopCollector(), 1),
	}

	channel := channels.ConsensusCommittee
	payload := []byte("dropped-then-retransmitted-payload")

	// Two scopes carrying the identical channel+payload but from different origins:
	// originA's delivery attempt hits the queue-full window; originB is a different,
	// honest publisher retransmitting the same payload afterwards.
	newScope := func(origin flow.Identifier, p []byte) *message.IncomingMessageScope {
		scope, err := message.NewIncomingScope(
			origin,
			message.ProtocolTypePubSub,
			&message.Message{ChannelID: channel.String(), Payload: p},
			p)
		require.NoError(t, err)
		return scope
	}
	victimFromA := newScope(unittest.IdentifierFixture(), payload)
	victimFromB := newScope(unittest.IdentifierFixture(), payload)
	filler := newScope(unittest.IdentifierFixture(), []byte("filler-payload"))

	// The dedup event ID is sender-independent.
	require.Equal(t, victimFromA.EventID(), victimFromB.EventID(),
		"identical channel+payload from different senders yields the identical event ID")
	require.NotEqual(t, filler.EventID(), victimFromA.EventID())

	// Step 1: occupy the single queue slot.
	require.NoError(t, n.processNetworkMessage(filler))
	require.Equal(t, 1, n.queue.Len())

	// Step 2: the victim message arrives during the queue-full window and is dropped.
	metricsCollector.On("QueueFullInboundMessagesDropped", channel.String(), message.ProtocolTypePubSub.String(), victimFromA.PayloadType()).Once()
	err := n.processNetworkMessage(victimFromA)
	require.ErrorIs(t, err, queue.ErrQueueFull,
		"victim message is rejected because the inbound queue is full")
	require.Equal(t, 1, n.queue.Len(), "only the filler remains queued")

	// Step 3: the failed enqueue rolled back the dedup cache entry, so the event ID of the
	// dropped message is no longer present. Probing the cache with Add returns true (unseen).
	require.True(t, n.receiveCache.Add(victimFromA.EventID()),
		"event ID of a queue-full-dropped message must not remain in the dedup cache")

	// The cache Add above is just a probe; remove it to restore the prior cache state.
	require.True(t, n.receiveCache.Remove(victimFromA.EventID()),
		"probe entry must be removable")

	// Step 4: a queue worker drains the filler.
	require.NotNil(t, n.queue.Remove())
	require.Equal(t, 0, n.queue.Len())

	// Step 5: the identical payload is retransmitted by a different publisher and accepted.
	err = n.processNetworkMessage(victimFromB)
	require.NoError(t, err,
		"retransmission of a previously dropped message must be accepted")
	require.Equal(t, 1, n.queue.Len(),
		"retransmission must be enqueued so the engine can process it")
}
