package underlay

import (
	"bytes"
	"context"
	"testing"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/messages"
	"github.com/onflow/flow-go/module/metrics"
	modulemock "github.com/onflow/flow-go/module/mock"
	"github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/network/alsp"
	netcache "github.com/onflow/flow-go/network/cache"
	"github.com/onflow/flow-go/network/channels"
	"github.com/onflow/flow-go/network/codec"
	"github.com/onflow/flow-go/network/codec/cbor"
	"github.com/onflow/flow-go/network/message"
	mockmsg "github.com/onflow/flow-go/network/mock"
	p2plogging "github.com/onflow/flow-go/network/p2p/logging"
	"github.com/onflow/flow-go/network/queue"
	"github.com/onflow/flow-go/network/slashing"
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

// stubIDTranslator implements the p2p.IDTranslator interface used by the Network for tests.
// It only needs to translate a peer ID back to the configured Flow ID.
type stubIDTranslator struct {
	id flow.Identifier
}

func (t stubIDTranslator) GetPeerID(_ flow.Identifier) (peer.ID, error) {
	return "", nil
}

func (t stubIDTranslator) GetFlowID(_ peer.ID) (flow.Identifier, error) {
	return t.id, nil
}

// newTestNetwork returns a Network with the minimal field set needed to drive
// processAuthenticatedMessage in tests.
func newTestNetwork(
	t *testing.T,
	idProvider *modulemock.IdentityProvider,
	metrics *modulemock.NetworkSecurityMetrics,
	reportConsumer *mockmsg.MisbehaviorReportConsumer,
	originID flow.Identifier,
) *Network {
	return &Network{
		logger:                     unittest.Logger(),
		identityProvider:           idProvider,
		identityTranslator:         stubIDTranslator{id: originID},
		codec:                      cbor.NewCodec(),
		slashingViolationsConsumer: slashing.NewSlashingViolationsConsumer(unittest.Logger(), metrics, reportConsumer),
	}
}

// TestProcessAuthenticatedMessage_ReportsDecodeFailureOnStakedChannel verifies that a staked peer
// sending a message with a valid message-code byte but an undecodable payload is reported to ALSP
// so that a penalty can be applied.
func TestProcessAuthenticatedMessage_ReportsDecodeFailureOnStakedChannel(t *testing.T) {
	attackerIdentity := unittest.IdentityFixture(
		unittest.WithRole(flow.RoleConsensus),
		unittest.WithParticipationStatus(flow.EpochParticipationStatusActive),
	)
	attackerPeerID := unittest.PeerIdFixture(t)
	channel := channels.ConsensusCommittee

	// A valid message-code byte followed by undecodable CBOR garbage. This passes the
	// authorized-sender check (which only looks at the code byte) but fails codec.Decode.
	payload := append([]byte{codec.CodeBlockProposal.Uint8()}, bytes.Repeat([]byte{0xFF}, 32)...)

	reportConsumer := mockmsg.NewMisbehaviorReportConsumer(t)
	var reported []network.MisbehaviorReport
	reportConsumer.On("ReportMisbehaviorOnChannel", channel, mock.Anything).
		Run(func(args mock.Arguments) {
			reported = append(reported, args.Get(1).(network.MisbehaviorReport))
		}).
		Once()

	metrics := modulemock.NewNetworkSecurityMetrics(t)
	metrics.On("OnUnauthorizedMessage", attackerIdentity.Role.String(), "unknown", channel.String(), alsp.InvalidMessage.String()).Once()

	idProvider := modulemock.NewIdentityProvider(t)
	idProvider.On("ByPeerID", attackerPeerID).Return(attackerIdentity, true).Once()

	net := newTestNetwork(t, idProvider, metrics, reportConsumer, attackerIdentity.NodeID)

	net.processAuthenticatedMessage(&message.Message{ChannelID: channel.String(), Payload: payload}, attackerPeerID, message.ProtocolTypePubSub)

	require.Len(t, reported, 1, "ALSP report must be submitted for a staked peer that sends an undecodable message")
	require.Equal(t, attackerIdentity.NodeID, reported[0].OriginId())
	require.Equal(t, alsp.InvalidMessage, reported[0].Reason())
}

// TestProcessAuthenticatedMessage_ReportsToInternalFailureOnStakedChannel verifies that a staked
// peer sending well-formed CBOR that decodes successfully but fails structural validation in
// ToInternal (e.g. a proposal with an empty chain ID) is also reported to ALSP.
func TestProcessAuthenticatedMessage_ReportsToInternalFailureOnStakedChannel(t *testing.T) {
	attackerIdentity := unittest.IdentityFixture(
		unittest.WithRole(flow.RoleConsensus),
		unittest.WithParticipationStatus(flow.EpochParticipationStatusActive),
	)
	attackerPeerID := unittest.PeerIdFixture(t)
	channel := channels.ConsensusCommittee

	// Well-formed CBOR that decodes into a proposal but fails structural validation in
	// ToInternal (the empty proposal has an empty chain ID). Codec.Encode already prepends
	// the message-code byte, so no manual prepend here.
	cborCodec := cbor.NewCodec()
	payload, err := cborCodec.Encode(&messages.Proposal{})
	require.NoError(t, err)

	// Pin the payload's properties: it must decode cleanly and fail in ToInternal, otherwise
	// this test silently covers the unmarshal branch instead.
	decoded, err := cborCodec.Decode(payload)
	require.NoError(t, err, "payload must decode; this test covers the ToInternal branch")
	_, err = decoded.ToInternal()
	require.Error(t, err, "payload must fail structural validation in ToInternal")

	reportConsumer := mockmsg.NewMisbehaviorReportConsumer(t)
	var reported []network.MisbehaviorReport
	reportConsumer.On("ReportMisbehaviorOnChannel", channel, mock.Anything).
		Run(func(args mock.Arguments) {
			reported = append(reported, args.Get(1).(network.MisbehaviorReport))
		}).
		Once()

	metrics := modulemock.NewNetworkSecurityMetrics(t)
	metrics.On("OnUnauthorizedMessage", attackerIdentity.Role.String(), "unknown", channel.String(), alsp.InvalidMessage.String()).Once()

	idProvider := modulemock.NewIdentityProvider(t)
	idProvider.On("ByPeerID", attackerPeerID).Return(attackerIdentity, true).Once()

	net := newTestNetwork(t, idProvider, metrics, reportConsumer, attackerIdentity.NodeID)

	net.processAuthenticatedMessage(&message.Message{ChannelID: channel.String(), Payload: payload}, attackerPeerID, message.ProtocolTypePubSub)

	require.Len(t, reported, 1, "ALSP report must be submitted for a staked peer that sends a structurally invalid message")
	require.Equal(t, attackerIdentity.NodeID, reported[0].OriginId())
	require.Equal(t, alsp.InvalidMessage, reported[0].Reason())
}

// TestProcessAuthenticatedMessage_SkipsPublicChannelDecodeFailure verifies that decode failures on
// public channels are not reported to ALSP, preserving the existing exemption for the public network.
func TestProcessAuthenticatedMessage_SkipsPublicChannelDecodeFailure(t *testing.T) {
	peerID := unittest.PeerIdFixture(t)
	channel := channels.PublicReceiveBlocks

	// Same undecodable payload as the staked-channel test.
	payload := append([]byte{codec.CodeBlockProposal.Uint8()}, bytes.Repeat([]byte{0xFF}, 32)...)

	reportConsumer := mockmsg.NewMisbehaviorReportConsumer(t)
	// No ReportMisbehaviorOnChannel expectation: mock strictness is the assertion that the
	// violation must be skipped for public channels.

	metrics := modulemock.NewNetworkSecurityMetrics(t)
	metrics.On("OnUnauthorizedMessage", "unknown", "unknown", channel.String(), alsp.InvalidMessage.String()).Once()
	metrics.On("OnViolationReportSkipped").Once()

	net := newTestNetwork(t, modulemock.NewIdentityProvider(t), metrics, reportConsumer, unittest.IdentifierFixture())

	net.processAuthenticatedMessage(&message.Message{ChannelID: channel.String(), Payload: payload}, peerID, message.ProtocolTypePubSub)
}
