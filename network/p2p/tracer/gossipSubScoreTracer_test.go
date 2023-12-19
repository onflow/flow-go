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

	"github.com/onflow/flow-go/config"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/module/metrics"
	mockmodule "github.com/onflow/flow-go/module/mock"
	"github.com/onflow/flow-go/network/channels"
	"github.com/onflow/flow-go/network/p2p"
	p2ptest "github.com/onflow/flow-go/network/p2p/test"
	"github.com/onflow/flow-go/network/p2p/tracer"
	validator "github.com/onflow/flow-go/network/validator/pubsub"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestGossipSubScoreTracer tests the functionality of the GossipSubScoreTracer, which logs the scores
// of the libp2p nodes using the GossipSub protocol. The test sets up three nodes with the same role,
// and subscribes them to a common topic. One of these three nodes is furnished with the score tracer, and the test
// examines whether the tracer node is able to trace the local score of other two nodes properly.
// The test also checks that the correct metrics are being called for each score update.
//
// The test performs the following steps:
// 1. Creates a logger hook to count the number of times the score logs at the interval specified.
// 2. Creates a mockPeerScoreMetrics object and sets it as a metrics collector for the tracer node.
// 3. Creates three nodes with same roles and sets their roles as consensus, access, and tracer, respectively.
// 4. Sets some fixed scores for the nodes for the sake of testing based on their roles.
// 5. Starts the nodes and lets them discover each other.
// 6. Subscribes the nodes to a common topic.
// 7. Expects the tracer node to have the correct app scores, a non-zero score, an existing behaviour score, an existing
// IP score, and an existing mesh score.
// 8. Expects the score tracer to log the scores at least once.
// 9. Checks that the correct metrics are being called for each score update.
func TestGossipSubScoreTracer(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	signalerCtx := irrecoverable.NewMockSignalerContext(t, ctx)
	sporkId := unittest.IdentifierFixture()
	idProvider := mockmodule.NewIdentityProvider(t)
	defer cancel()

	loggerCycle := atomic.NewInt32(0)

	// 1. Creates a logger hook to count the number of times the score logs at the interval specified.
	hook := zerolog.HookFunc(func(e *zerolog.Event, level zerolog.Level, message string) {
		if level == zerolog.DebugLevel {
			if message == tracer.PeerScoreLogMessage {
				loggerCycle.Inc()
			}
		}
	})
	logger := zerolog.New(os.Stdout).Level(zerolog.DebugLevel).Hook(hook)

	// sets some fixed scores for the nodes for sake of testing based on their roles.
	consensusScore := float64(87)
	accessScore := float64(77)

	// 2. Creates a mockPeerScoreMetrics object and sets it as a metrics collector for the tracer node.
	scoreMetrics := mockmodule.NewGossipSubScoringMetrics(t)
	topic1 := channels.TopicFromChannel(channels.PushBlocks, sporkId)

	// 3. Creates three nodes with different roles and sets their roles as consensus, access, and tracer, respectively.
	cfg, err := config.DefaultConfig()
	require.NoError(t, err)
	// tracer will update the score and local mesh every 1 second (for testing purposes)
	cfg.NetworkConfig.GossipSub.RpcTracer.LocalMeshLogInterval = 1 * time.Second
	cfg.NetworkConfig.GossipSub.RpcTracer.ScoreTracerInterval = 1 * time.Second
	// the libp2p node updates the subscription list as well as the app-specific score every 10 milliseconds (for testing purposes)
	cfg.NetworkConfig.GossipSub.SubscriptionProvider.UpdateInterval = 10 * time.Millisecond
	cfg.NetworkConfig.GossipSub.ScoringParameters.AppSpecificScore.ScoreTTL = 10 * time.Millisecond
	tracerNode, tracerId := p2ptest.NodeFixture(
		t,
		sporkId,
		t.Name(),
		idProvider,
		p2ptest.WithMetricsCollector(&mockPeerScoreMetrics{
			NoopCollector: metrics.NoopCollector{},
			c:             scoreMetrics,
		}),
		p2ptest.WithLogger(logger),
		p2ptest.OverrideFlowConfig(cfg),
		p2ptest.EnablePeerScoringWithOverride(&p2p.PeerScoringConfigOverride{
			AppSpecificScoreParams: func(pid peer.ID) float64 {
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
			},
			TopicScoreParams: map[channels.Topic]*pubsub.TopicScoreParams{
				topic1: {
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
				},
			},
		}),
		p2ptest.WithRole(flow.RoleConsensus))

	idProvider.On("ByPeerID", tracerNode.ID()).Return(&tracerId, true).Maybe()

	consensusNode, consensusId := p2ptest.NodeFixture(
		t,
		sporkId,
		t.Name(),
		idProvider,
		p2ptest.WithRole(flow.RoleConsensus))
	idProvider.On("ByPeerID", consensusNode.ID()).Return(&consensusId, true).Maybe()

	accessNode, accessId := p2ptest.NodeFixture(
		t,
		sporkId,
		t.Name(),
		idProvider,
		p2ptest.WithRole(flow.RoleAccess))
	idProvider.On("ByPeerID", accessNode.ID()).Return(&accessId, true).Maybe()

	nodes := []p2p.LibP2PNode{tracerNode, consensusNode, accessNode}
	ids := flow.IdentityList{&tracerId, &consensusId, &accessId}

	// 5. Starts the nodes and lets them discover each other.
	p2ptest.StartNodes(t, signalerCtx, nodes)
	defer p2ptest.StopNodes(t, nodes, cancel)

	p2ptest.LetNodesDiscoverEachOther(t, ctx, nodes, ids)

	// 9. Checks that the correct metrics are being called for each score update.
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

	// 6. Subscribes the nodes to a common topic.
	_, err = tracerNode.Subscribe(
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

	// 7. Expects the tracer node to have the correct app scores, a non-zero score, an existing behaviour score, an existing
	// IP score, and an existing mesh score.
	require.Eventually(t, func() bool {
		// we expect the tracerNode to have the consensusNodes and accessNodes with the correct app scores.
		exposer := tracerNode.PeerScoreExposer()
		score, ok := exposer.GetAppScore(consensusNode.ID())
		if !ok || score != consensusScore {
			return false
		}

		score, ok = exposer.GetAppScore(accessNode.ID())
		if !ok || score != accessScore {
			return false
		}

		// we expect the tracerNode to have the consensusNodes and accessNodes with a non-zero score.
		score, ok = exposer.GetScore(consensusNode.ID())
		if !ok || score == 0 {
			return false
		}

		score, ok = exposer.GetScore(accessNode.ID())
		if !ok || score == 0 {
			return false
		}

		// we expect the tracerNode to have the consensusNodes and accessNodes with an existing behaviour score and ip score.
		_, ok = exposer.GetBehaviourPenalty(consensusNode.ID())
		if !ok {
			return false
		}

		_, ok = exposer.GetIPColocationFactor(consensusNode.ID())
		if !ok {
			return false
		}

		_, ok = exposer.GetBehaviourPenalty(accessNode.ID())
		if !ok {
			return false
		}

		_, ok = exposer.GetIPColocationFactor(accessNode.ID())
		if !ok {
			return false
		}

		// we expect the tracerNode to have the consensusNodes and accessNodes with an existing mesh score.
		consensusMeshScores, ok := exposer.GetTopicScores(consensusNode.ID())
		if !ok {
			return false
		}
		_, ok = consensusMeshScores[topic1.String()]
		if !ok {
			return false
		}

		accessMeshScore, ok := exposer.GetTopicScores(accessNode.ID())
		if !ok {
			return false
		}
		_, ok = accessMeshScore[topic1.String()]
		return ok
	}, 2*time.Second, 10*time.Millisecond)

	time.Sleep(2 * time.Second)

	// 8. Expects the score tracer to log the scores at least once.
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
