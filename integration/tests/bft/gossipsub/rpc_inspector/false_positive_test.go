package rpc_inspector

import (
	"testing"

	"github.com/stretchr/testify/suite"
)

const numOfTestAccounts = 5

type GossipsubRPCInstpectorFalePostiveNotificationsTestSuite struct {
	Suite
}

func TestGossipSubRpcInspectorFalsePositiveNotifications(t *testing.T) {
	suite.Run(t, new(GossipsubRPCInstpectorFalePostiveNotificationsTestSuite))
}

// TestGossipsubRPCInstpectorFalePostiveNotifications this test ensures that any underlying changes or updates to any of the underlying libp2p libraries
// does not result in any of the gossip sub rpc control message inspector validation rules being broken. Anytime a validation rule is broken an invalid
// control message notification is disseminated. Using this fact, this tests sets up a full flow network and submits some transactions to generate network
// activity. After some time we ensure that no invalid control message notifications are disseminated.
func (s *GossipsubRPCInstpectorFalePostiveNotificationsTestSuite) TestGossipsubRPCInstpectorFalePostiveNotifications() {
	// the network has started submit some transactions to create flow accounts.
	// We wait for each of these transactions to be sealed ensuring we generate
	// some artificial network activity
	for i := 0; i < numOfTestAccounts; i++ {
		s.submitSmokeTestTransaction(s.Ctx)
	}
	// ensure no node in the network has disseminated an invalid control message notification
	metricName := s.inspectorNotifQSizeMetricName()
	metricsByContainer := s.Net.GetMetricFromContainers(s.T(), metricName, s.metricsUrls())
	s.ensureNoNotificationsDisseminated(metricsByContainer)
}
