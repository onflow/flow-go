package topicvalidator

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	
	"github.com/onflow/flow-go/utils/unittest"
)

type TopicValidatorTestSuite struct {
	Suite
}

func TestTopicValidator(t *testing.T) {
	suite.Run(t, new(TopicValidatorTestSuite))
}

// TestTopicValidatorE2E ensures that the libp2p topic validator is working as expected.
// This test will attempt to send multiple combinations of unauthorized messages + channel from
// a corrupted byzantine attacker node. The victim node should not receive any of these messages as they should
// be dropped due to failing message authorization validation at the topic validator. This test will also send
// a number of authorized messages that will be delivered and processed by the victim node, ensuring that the topic
// validator behaves as expected in the happy path.
func (s *TopicValidatorTestSuite) TestTopicValidatorE2E() {
	s.Orchestrator.sendUnauthorizedMsgs(s.T())
	s.Orchestrator.sendAuthorizedMsgs(s.T())
	unittest.RequireReturnsBefore(s.T(), s.Orchestrator.authorizedEventsWg.Wait, 5*time.Second, "could not send authorized messages on time")
	require.Equal(s.T(), 0, s.Orchestrator.unauthorizedEventsReceived.Len())
	require.Equal(s.T(), numOfAuthorizedEvents, s.Orchestrator.authorizedEventsReceived.Len())
}
