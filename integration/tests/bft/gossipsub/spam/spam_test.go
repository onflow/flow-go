package spam

import (
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/utils/unittest"
)

type GossipSubSpamMitigationIntegrationTestSuite struct {
	Suite
}

func TestGossipSubSpamMitigationSuite(t *testing.T) {
	suite.Run(t, new(GossipSubSpamMitigationIntegrationTestSuite))
}

func (s *GossipSubSpamMitigationIntegrationTestSuite) TestGossipSubWhisper() {
	s.orchestrator.sendOneEgressMessage(s.T())
	unittest.RequireReturnsBefore(s.T(), s.orchestrator.egressEventReceivedWg.Wait, 5*time.Second, "could not send messages on time")
}
