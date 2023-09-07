package disallowlisting

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/utils/unittest"
)

type AdminCommandDisallowListTestSuite struct {
	Suite
}

func TestAdminCommandDisallowList(t *testing.T) {
	suite.Run(t, new(AdminCommandDisallowListTestSuite))
}

// TestAdminCommandDisallowList ensures that the disallow-list admin command works as expected. When a node is blocked via the admin disallow-list command
// the libp2p connection to that node should be pruned immediately and the connection gater should start to block incoming connection requests. This test
// sets up 2 corrupt nodes a sender and receiver, the sender will send messages before and after being blocked by the receiver node via
// the disallow-list admin command. The receiver node is expected to receive messages like normal before disallow-listing the sender, after disallow-listing the sender
// it should not receive any messages. The reason this test is conducted via two corrupt nodes is to empower the test logic to command one (corrupt) node to send a message and
// to examine the other (corrupt) node to check whether it has received the message.
func (a *AdminCommandDisallowListTestSuite) TestAdminCommandDisallowList() {
	// send some authorized messages indicating the network is working as expected
	a.Orchestrator.sendAuthorizedMsgs(a.T())
	unittest.RequireReturnsBefore(a.T(), a.Orchestrator.authorizedEventsReceivedWg.Wait, 5*time.Second, "could not receive authorized messages on time")
	// messages with correct message signatures are expected to always pass libp2p signature verification and be delivered to the victim EN.
	require.Equal(
		a.T(),
		int64(numOfAuthorizedEvents),
		a.Orchestrator.authorizedEventsReceived.Load(),
		fmt.Sprintf("expected to receive %d authorized events got: %d", numOfAuthorizedEvents, a.Orchestrator.expectedBlockedEventsReceived.Load()))

	// after disallow-listing node (a.senderVN) we should not receive any messages from that node.
	// This is an asynchronous process with a number of sub processes involved including but not limited to:
	// - submitting request to admin server for node to be disallow-listed.
	// - node disallow-list must update.
	// - peer manager needs to prune the connection (takes at least 1 second).
	// - connection gater will start blocking incoming connections (takes at least 1 second).
	// We wait for 5 seconds to reduce the small chance of a race condition between the time a node is blocked
	// and the time the blocked node sends the first unauthorized message.
	a.disallowListNode(a.senderVN)
	time.Sleep(5 * time.Second)

	// send unauthorized messages and sleep for 3 seconds to allow all requests to be processed
	// in normal situations if the node is not disallow-listed, these messages would be considered
	// legit and hence would be delivered to the recipients.
	a.Orchestrator.sendExpectedBlockedMsgs(a.T())

	// The sleep is unavoidable for the following reasons these messages are sent between 2 running libp2p nodes we don't have any hooks in between
	// These are messages sent after the node is blocked meaning that these messages are not expected to be delivered to the receiver node,
	// so we sleep this approximate amount of time to ensure all messages were attempted, processed and dropped.
	time.Sleep(3 * time.Second)

	// messages sent after the node is disallow-listed are considered unauthorized, we don't expect to receive any of them.
	require.Equal(
		a.T(),
		int64(0),
		a.Orchestrator.expectedBlockedEventsReceived.Load(),
		fmt.Sprintf("expected to not receive any unauthorized messages instead got: %d", a.Orchestrator.expectedBlockedEventsReceived.Load()))
}
