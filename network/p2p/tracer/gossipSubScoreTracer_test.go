package tracer_test

import (
	"context"
	"os"
	"testing"
	"time"

	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/atomic"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/module/metrics"
	mockmodule "github.com/onflow/flow-go/module/mock"
	"github.com/onflow/flow-go/network/channels"
	"github.com/onflow/flow-go/network/p2p"
	"github.com/onflow/flow-go/network/p2p/scoring"
	p2ptest "github.com/onflow/flow-go/network/p2p/test"
	"github.com/onflow/flow-go/network/p2p/tracer"
	validator "github.com/onflow/flow-go/network/validator/pubsub"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestGossipSubScoreTracer(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	signalerCtx := irrecoverable.NewMockSignalerContext(t, ctx)
	sporkId := unittest.IdentifierFixture()
	idProvider := mockmodule.NewIdentityProvider(t)
	defer cancel()

	loggerCycle := atomic.NewInt32(0)

	// logger hook to count the number of times the score logs at the interval specified.
	hook := zerolog.HookFunc(func(e *zerolog.Event, level zerolog.Level, message string) {
		if level == zerolog.InfoLevel {
			if message == tracer.PeerScoreLogMessage {
				loggerCycle.Inc()
			}
		}
	})
	logger := zerolog.New(os.Stdout).Level(zerolog.InfoLevel).Hook(hook)

	// sets some fixed scores for the nodes for sake of testing based on their roles.
	consensusScore := float64(87)
	accessScore := float64(77)

	scoreMetrics := mockmodule.NewGossipSubScoringMetrics(t)
	topic1 := channels.TopicFromChannel(channels.PushBlocks, sporkId)

	tracerNode, tracerId := p2ptest.NodeFixture(
		t,
		sporkId,
		t.Name(),
		p2ptest.WithMetricsCollector(&mockPeerScoreMetrics{
			NoopCollector: metrics.NoopCollector{},
			c:             scoreMetrics,
		}),
		p2ptest.WithLogger(logger),
		p2ptest.WithPeerScoreTracerInterval(1*time.Second), // set the peer score log interval to 1 second for sake of testing.
		p2ptest.WithPeerScoringEnabled(idProvider),         // enable peer scoring for sake of testing.
		p2ptest.WithPeerScoreParamsOption(
			scoring.WithAppSpecificScoreFunction(
				func(pid peer.ID) float64 {
					id, ok := idProvider.ByPeerID(pid)
					require.True(t, ok)

					switch id.Role {
					case flow.RoleConsensus:
						return consensusScore
					case flow.RoleAccess:
						return accessScore
					default:
						t.Fatalf("unexpected role: %s", id.Role)
					}
					return 0
				})),
		p2ptest.WithPeerScoreParamsOption(scoring.WithTopicScoreParams(topic1, &pubsub.TopicScoreParams{
			// set the topic score params to some fixed values for sake of testing.
			// Note that these values are not realistic and should not be used in production.
			TopicWeight:                     1,
			TimeInMeshQuantum:               1 * time.Second,
			TimeInMeshWeight:                1,
			TimeInMeshCap:                   1000,
			FirstMessageDeliveriesWeight:    1,
			FirstMessageDeliveriesDecay:     0.999,
			FirstMessageDeliveriesCap:       1000,
			MeshMessageDeliveriesWeight:     -1,
			MeshMessageDeliveriesDecay:      0.999,
			MeshMessageDeliveriesThreshold:  100,
			MeshMessageDeliveriesActivation: 1 * time.Second,
			MeshMessageDeliveriesCap:        1000,
			MeshFailurePenaltyWeight:        -1,
			MeshFailurePenaltyDecay:         0.999,
			InvalidMessageDeliveriesWeight:  -1,
			InvalidMessageDeliveriesDecay:   0.999,
		})),
		p2ptest.WithRole(flow.RoleConsensus))

	idProvider.On("ByPeerID", tracerNode.Host().ID()).Return(&tracerId, true).Maybe()

	consensusNode, consensusId := p2ptest.NodeFixture(
		t,
		sporkId,
		t.Name(),
		p2ptest.WithRole(flow.RoleConsensus))
	idProvider.On("ByPeerID", consensusNode.Host().ID()).Return(&consensusId, true).Maybe()

	accessNode, accessId := p2ptest.NodeFixture(
		t,
		sporkId,
		t.Name(),
		p2ptest.WithRole(flow.RoleAccess))
	idProvider.On("ByPeerID", accessNode.Host().ID()).Return(&accessId, true).Maybe()

	nodes := []p2p.LibP2PNode{tracerNode, consensusNode, accessNode}
	ids := flow.IdentityList{&tracerId, &consensusId, &accessId}

	p2ptest.StartNodes(t, signalerCtx, nodes, 1*time.Second)
	defer p2ptest.StopNodes(t, nodes, cancel, 1*time.Second)

	p2ptest.LetNodesDiscoverEachOther(t, ctx, nodes, ids)

	scoreMetrics.On("OnOverallPeerScoreUpdated", mock.Anything).Return()
	scoreMetrics.On("OnAppSpecificScoreUpdated", mock.Anything).Return()
	scoreMetrics.On("OnIPColocationFactorUpdated", mock.Anything).Return()
	scoreMetrics.On("OnBehaviourPenaltyUpdated", mock.Anything).Return()
	scoreMetrics.On("OnTimeInMeshUpdated", topic1, mock.Anything).Return()
	scoreMetrics.On("OnFirstMessageDeliveredUpdated", topic1, mock.Anything).Return()
	scoreMetrics.On("OnMeshMessageDeliveredUpdated", topic1, mock.Anything).Return()
	scoreMetrics.On("OnMeshMessageDeliveredUpdated", topic1, mock.Anything).Return()
	scoreMetrics.On("OnInvalidMessageDeliveredUpdated", topic1, mock.Anything).Return()
	scoreMetrics.On("SetWarningStateCount", uint(0)).Return()

	_, err := tracerNode.Subscribe(
		topic1,
		validator.TopicValidator(
			unittest.Logger(),
			unittest.AllowAllPeerFilter()))
	require.NoError(t, err)

	_, err = consensusNode.Subscribe(
		topic1,
		validator.TopicValidator(
			unittest.Logger(),
			unittest.AllowAllPeerFilter()))
	require.NoError(t, err)

	_, err = accessNode.Subscribe(
		topic1,
		validator.TopicValidator(
			unittest.Logger(),
			unittest.AllowAllPeerFilter()))
	require.NoError(t, err)

	assert.Eventually(t, func() bool {
		// we expect the tracerNode to have the consensusNodes and accessNodes with the correct app scores.
		score, ok := tracerNode.PeerScoreExposer().GetAppScore(consensusNode.Host().ID())
		if !ok || score != consensusScore {
			return false
		}

		score, ok = tracerNode.PeerScoreExposer().GetAppScore(accessNode.Host().ID())
		if !ok || score != accessScore {
			return false
		}

		// we expect the tracerNode to have the consensusNodes and accessNodes with a non-zero score.
		score, ok = tracerNode.PeerScoreExposer().GetScore(consensusNode.Host().ID())
		if !ok || score == 0 {
			return false
		}

		score, ok = tracerNode.PeerScoreExposer().GetScore(accessNode.Host().ID())
		if !ok || score == 0 {
			return false
		}

		// we expect the tracerNode to have the consensusNodes and accessNodes with an existing behaviour score and ip score.
		_, ok = tracerNode.PeerScoreExposer().GetBehaviourPenalty(consensusNode.Host().ID())
		if !ok {
			return false
		}

		_, ok = tracerNode.PeerScoreExposer().GetIPColocationFactor(consensusNode.Host().ID())
		if !ok {
			return false
		}

		_, ok = tracerNode.PeerScoreExposer().GetBehaviourPenalty(accessNode.Host().ID())
		if !ok {
			return false
		}

		_, ok = tracerNode.PeerScoreExposer().GetIPColocationFactor(accessNode.Host().ID())
		if !ok {
			return false
		}

		// we expect the tracerNode to have the consensusNodes and accessNodes with an existing mesh score.
		consensusMeshScores, ok := tracerNode.PeerScoreExposer().GetTopicScores(consensusNode.Host().ID())
		if !ok {
			return false
		}
		_, ok = consensusMeshScores[topic1.String()]
		if !ok {
			return false
		}

		accessMeshScore, ok := tracerNode.PeerScoreExposer().GetTopicScores(accessNode.Host().ID())
		if !ok {
			return false
		}
		_, ok = accessMeshScore[topic1.String()]
		return ok
	}, 2*time.Second, 10*time.Millisecond)

	time.Sleep(2 * time.Second)

	// eventually, we expect the score tracer to log the score at least once.
	assert.Eventually(t, func() bool {
		return loggerCycle.Load() > 0
	}, 2*time.Second, 10*time.Millisecond)
}

type mockPeerScoreMetrics struct {
	metrics.NoopCollector
	c module.GossipSubScoringMetrics
}

func (m *mockPeerScoreMetrics) OnOverallPeerScoreUpdated(f float64) {
	m.c.OnOverallPeerScoreUpdated(f)
}

func (m *mockPeerScoreMetrics) OnAppSpecificScoreUpdated(f float64) {
	m.c.OnAppSpecificScoreUpdated(f)
}

func (m *mockPeerScoreMetrics) OnIPColocationFactorUpdated(f float64) {
	m.c.OnIPColocationFactorUpdated(f)
}

func (m *mockPeerScoreMetrics) OnBehaviourPenaltyUpdated(f float64) {
	m.c.OnBehaviourPenaltyUpdated(f)
}

func (m *mockPeerScoreMetrics) OnTimeInMeshUpdated(topic channels.Topic, duration time.Duration) {
	m.c.OnTimeInMeshUpdated(topic, duration)
}

func (m *mockPeerScoreMetrics) OnFirstMessageDeliveredUpdated(topic channels.Topic, f float64) {
	m.c.OnFirstMessageDeliveredUpdated(topic, f)
}

func (m *mockPeerScoreMetrics) OnMeshMessageDeliveredUpdated(topic channels.Topic, f float64) {
	m.c.OnMeshMessageDeliveredUpdated(topic, f)
}

func (m *mockPeerScoreMetrics) OnInvalidMessageDeliveredUpdated(topic channels.Topic, f float64) {
	m.c.OnInvalidMessageDeliveredUpdated(topic, f)
}

func (m *mockPeerScoreMetrics) SetWarningStateCount(u uint) {
	m.c.SetWarningStateCount(u)
}

var _ module.GossipSubScoringMetrics = (*mockPeerScoreMetrics)(nil)
