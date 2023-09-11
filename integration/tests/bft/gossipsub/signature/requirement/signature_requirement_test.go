package requirement

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/utils/unittest"
)

type GossipSubSignatureRequirementTestSuite struct {
	Suite
}

func TestGossipSubSignatureRequirement(t *testing.T) {
	suite.Run(t, new(GossipSubSignatureRequirementTestSuite))
}

// TestGossipSubSignatureRequirement ensures that the libp2p signature verification is working as expected.
// The test network is set up with 3 corrupted nodes, 2 attackers and one victim. 1 attacker VN that has message signing disabled, this
// node will be used to send messages.ChunkDataRequest without signatures to the victim EN. These messages should never be
// delivered to the victim as the signature verification is expected to fail. 1 attacker VN with message
// signing enabled, this node will send messages that are expected to be delivered to the victim EN.
// This tests ensures that message signing and message signature verification requirements are working as expected. It does not
// test for malformed message signatures or impersonation message signatures.
func (s *GossipSubSignatureRequirementTestSuite) TestGossipSubSignatureRequirement() {
	s.Orchestrator.sendUnauthorizedMsgs(s.T())
	s.Orchestrator.sendAuthorizedMsgs(s.T())
	unittest.RequireReturnsBefore(s.T(), s.Orchestrator.authorizedEventReceivedWg.Wait, 5*time.Second, "could not send authorized messages on time")

	// messages without correct signature should have all been rejected at the libp2p level and not be delivered to the victim EN.
	require.Equal(s.T(), int64(0), s.Orchestrator.unauthorizedEventsReceived.Load(), fmt.Sprintf("expected to not receive any unauthorized messages instead got: %d", s.Orchestrator.unauthorizedEventsReceived.Load()))

	// messages with correct message signatures are expected to always pass libp2p signature verification and be delivered to the victim EN.
	require.Eventually(s.T(), func() bool {
		return s.Orchestrator.authorizedEventsReceived.Load() == int64(numOfAuthorizedEvents)
	}, 5*time.Second, 500*time.Millisecond, fmt.Sprintf("expected to receive %d authorized events got: %d", numOfAuthorizedEvents, s.Orchestrator.unauthorizedEventsReceived.Load()))
}
