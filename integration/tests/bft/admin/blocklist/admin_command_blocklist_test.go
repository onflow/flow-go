package blocklist

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/utils/unittest"
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
	// send some authorized messages indicating the network is working as expected
	a.Orchestrator.sendAuthorizedMsgs(a.T())
	unittest.RequireReturnsBefore(a.T(), a.Orchestrator.authorizedEventsReceivedWg.Wait, 5*time.Second, "could not send authorized messages on time")
	// messages with correct message signatures are expected to always pass libp2p signature verification and be delivered to the victim EN.
	require.Equal(a.T(), int64(10), a.Orchestrator.authorizedEventsReceived.Load(), fmt.Sprintf("expected to receive %d authorized events got: %d", 10, a.Orchestrator.unauthorizedEventsReceived.Load()))

	// after blocking node a.senderVN we should not receive any messages from that node. We wait for
	// 500 milliseconds to reduce the small chance of a race condition between the time a node is blocked
	// and the time the blocked node sends the first unauthorized message
	a.blockNode(a.senderVN)
	time.Sleep(500 * time.Millisecond)

	// send unauthorized messages and sleep for 3 seconds to allow all requests to be processed
	a.Orchestrator.sendUnauthorizedMsgs(a.T())
	time.Sleep(3 * time.Second)

	// messages without correct signature should have all been rejected at the libp2p level and not be delivered to the victim EN.
	require.Equal(a.T(), int64(0), a.Orchestrator.unauthorizedEventsReceived.Load(), fmt.Sprintf("expected to not receive any unauthorized messages instead got: %d", a.Orchestrator.unauthorizedEventsReceived.Load()))
}
