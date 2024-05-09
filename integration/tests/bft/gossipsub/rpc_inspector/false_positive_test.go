package rpc_inspector

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
)

const numOfTestAccounts = 1000

type GossipsubRPCInspectorFalsePositiveNotificationsTestSuite struct {
	Suite
}

func TestGossipSubRpcInspectorFalsePositiveNotifications(t *testing.T) {
	suite.Run(t, new(GossipsubRPCInspectorFalsePositiveNotificationsTestSuite))
}

// TestGossipsubRPCInspectorFalsePositiveNotifications this test ensures that any underlying changes or updates to any of the underlying libp2p libraries
// does not result in any of the gossip sub rpc control message inspector validation rules being broken. Anytime a validation rule is broken an invalid
// control message notification is disseminated. Using this fact, this tests sets up a full flow network and submits some transactions to generate network
// activity. After some time we ensure that no invalid control message notifications are disseminated.
func (s *GossipsubRPCInspectorFalsePositiveNotificationsTestSuite) TestGossipsubRPCInspectorFalsePositiveNotifications() {
	loaderLoopDuration := 5 * time.Second
	loaderLoopInterval := 500 * time.Millisecond
	ctx, cancel := context.WithTimeout(s.Ctx, loaderLoopDuration)
	defer cancel()
	// the network has started submit some transactions to create flow accounts.
	// We wait for each of these transactions to be sealed ensuring we generate
	// some artificial network activity.
	go s.loaderLoop(ctx, numOfTestAccounts, loaderLoopInterval)
	// wait 20 state commitment changes, this ensures we simulated load on network as expected.
	s.waitForStateCommitments(s.Ctx, 3, time.Minute, 500*time.Millisecond)

	// ensure no node in the network has disseminated an invalid control message notification
	metricName := s.inspectorNotificationQSizeMetricName()
	metricsByContainer := s.Net.GetMetricFromContainers(s.T(), metricName, s.metricsUrls())
	s.ensureNoNotificationsDisseminated(metricsByContainer)
}
