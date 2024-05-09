package topicvalidator

import (
	"fmt"
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
	unittest.RequireReturnsBefore(
		s.T(),
		s.Orchestrator.authorizedEventReceivedWg.Wait,
		5*time.Second,
		"could not send authorized messages on time")

	// Victim nodes are configured with the topic validator enabled, therefore they should not have
	// received any of the unauthorized messages.
	require.Equal(
		s.T(),
		0,
		len(s.Orchestrator.unauthorizedEventsReceived),
		fmt.Sprintf("expected to not receive any unauthorized messages instead got: %d", len(s.Orchestrator.unauthorizedEventsReceived)))

	// Victim nodes should receive all the authorized events sent.
	require.Eventually(s.T(), func() bool {
		return len(s.Orchestrator.authorizedEventsReceived) == numOfAuthorizedEvents
	}, 5*time.Second, 500*time.Millisecond,
		fmt.Sprintf("expected to receive %d authorized events got: %d", numOfAuthorizedEvents, len(s.Orchestrator.unauthorizedEventsReceived)))
}
