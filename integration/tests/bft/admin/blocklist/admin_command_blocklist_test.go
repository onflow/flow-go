package blocklist

import (
	"fmt"
	"github.com/onflow/flow-go/utils/unittest"
	"github.com/stretchr/testify/require"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
)

type AdminCommandBlockListTestSuite struct {
	Suite
}

func TestAdminCommandBlockList(t *testing.T) {
	suite.Run(t, new(AdminCommandBlockListTestSuite))
}

// TestAdminCommandBlockList ensures that the blocklist admin command works as expected. When a node is blocked via the admin blocklist command
// the libp2p connection to that node should be pruned immediately and the connection should start to block incoming connection request. This test
// sets up 2 corrupt nodes a sender and receiver, the sender will send messages before and after being blocked by the receiver node via
// the blocklist admin command. The receiver node is expected to receive messages like normal before blocking the sender, after blocking the sender
// it should not receive any messages.
func (a *AdminCommandBlockListTestSuite) TestAdminCommandBlockList() {
	a.Orchestrator.sendUnauthorizedMsgs(a.T())
	a.Orchestrator.sendAuthorizedMsgs(a.T())
	unittest.RequireReturnsBefore(a.T(), a.Orchestrator.authorizedEventsReceivedWg.Wait, 5*time.Second, "could not send authorized messages on time")

	// messages without correct signature should have all been rejected at the libp2p level and not be delivered to the victim EN.
	require.Equal(a.T(), int64(0), a.Orchestrator.unauthorizedEventsReceived.Load(), fmt.Sprintf("expected to not receive any unauthorized messages instead got: %d", a.Orchestrator.unauthorizedEventsReceived.Load()))

	// messages with correct message signatures are expected to always pass libp2p signature verification and be delivered to the victim EN.
	require.Equal(a.T(), int64(10), a.Orchestrator.authorizedEventsReceived.Load(), fmt.Sprintf("expected to receive %d authorized events got: %d", 10, a.Orchestrator.unauthorizedEventsReceived.Load()))
}
