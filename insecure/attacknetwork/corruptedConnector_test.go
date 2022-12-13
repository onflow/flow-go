package attacknetwork

import (
	"context"
	"fmt"
	"math/rand"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/insecure"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/libp2p/message"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestConnectorHappy_Send checks that a CorruptedConnector can successfully create a connection to a remote Corruptible Conduit Factory (CCF).
// Moreover, it checks that the resulted connection is capable of intact message delivery in a timely fashion from attacker to CCF.
func TestConnectorHappyPath_Send(t *testing.T) {
	withMockCorruptibleConduitFactory(t, func(corruptedId flow.Identity, ctx irrecoverable.SignalerContext, ccf *mockCorruptibleConduitFactory) {
		// extracting port that ccf gRPC server is running on CCF.
		_, ccfPortStr, err := net.SplitHostPort(ccf.ServerAddress())
		require.NoError(t, err)

		connector := NewCorruptedConnector(unittest.Logger(),
			flow.IdentityList{&corruptedId},
			map[flow.Identifier]string{corruptedId.NodeID: ccfPortStr})
		// empty incoming handler, as this test does not evaluate receive path
		connector.WithIncomingMessageHandler(func(i *insecure.Message) {})

		// goroutine checks the mock ccf for receiving the attacker registration from connector.
		// the attacker registration arrives as the connector attempts a connection on to the ccf.
		attackerRegistered := make(chan struct{})
		go func() {
			<-ccf.attackerRegMsg
			close(attackerRegistered)
		}()

		// goroutine checks mock ccf for receiving the message sent over the connection.
		msg, _, _ := insecure.EgressMessageFixture(t, unittest.NetworkCodec(), insecure.Protocol_MULTICAST, &message.TestMessage{
			Text: fmt.Sprintf("this is a test message from attacker to ccf: %d", rand.Int()),
		})
		sentMsgReceived := make(chan struct{})
		go func() {
			receivedMsg := <-ccf.attackerMsg

			// received message should have an exact match on the relevant fields.
			// Note: only fields filled by test fixtures are checked, as some others
			// are filled by gRPC on the fly, which are not relevant to the test's sanity.
			require.Equal(t, receivedMsg.Egress.Payload, msg.Egress.Payload)
			require.Equal(t, receivedMsg.Egress.Protocol, msg.Egress.Protocol)
			require.Equal(t, receivedMsg.Egress.OriginID, msg.Egress.OriginID)
			require.Equal(t, receivedMsg.Egress.TargetNum, msg.Egress.TargetNum)
			require.Equal(t, receivedMsg.Egress.TargetIDs, msg.Egress.TargetIDs)
			require.Equal(t, receivedMsg.Egress.ChannelID, msg.Egress.ChannelID)

			close(sentMsgReceived)
		}()

		// creates a connection to the corruptible conduit ccf.
		connection, err := connector.Connect(ctx, corruptedId.NodeID)
		require.NoError(t, err)
		defer func() {
			require.NoError(t, connection.CloseConnection())
		}()

		// sends a message to ccf
		require.NoError(t, connection.SendMessage(msg))

		// checks a timely arrival of the registration and sent message at the ccf.
		unittest.RequireCloseBefore(t, attackerRegistered, 1*time.Second, "ccf could not receive attacker registration on time")
		// imitates sending a message from ccf to attacker through corrupted connection.
		unittest.RequireCloseBefore(t, sentMsgReceived, 1*time.Second, "ccf could not receive message sent on connection on time")
	})
}

// TestConnectorHappy_Receive checks that a CorruptedConnector can successfully create a connection to a remote Corruptible Conduit Factory (
// CCF).
// Moreover, it checks that the resulted connection is capable of intact message delivery in a timely fashion from CCF to attacker.
func TestConnectorHappyPath_Receive(t *testing.T) {
	withMockCorruptibleConduitFactory(t, func(corruptedId flow.Identity, ctx irrecoverable.SignalerContext, ccf *mockCorruptibleConduitFactory) {
		// extracting port that ccf gRPC server is running on
		_, ccfPortStr, err := net.SplitHostPort(ccf.ServerAddress())
		require.NoError(t, err)

		msg, _, _ := insecure.EgressMessageFixture(t, unittest.NetworkCodec(), insecure.Protocol_MULTICAST, &message.TestMessage{
			Text: fmt.Sprintf("this is a test message from ccf to attacker: %d", rand.Int()),
		})

		sentMsgReceived := make(chan struct{})
		connector := NewCorruptedConnector(unittest.Logger(),
			flow.IdentityList{&corruptedId},
			map[flow.Identifier]string{corruptedId.NodeID: ccfPortStr})
		connector.WithIncomingMessageHandler(
			func(receivedMsg *insecure.Message) {
				// received message by attacker should have an exact match on the relevant fields as sent by ccf.
				// Note: only fields filled by test fixtures are checked, as some others
				// are filled by gRPC on the fly, which are not relevant to the test's sanity.
				require.Equal(t, receivedMsg.Egress.Payload, msg.Egress.Payload)
				require.Equal(t, receivedMsg.Egress.Protocol, msg.Egress.Protocol)
				require.Equal(t, receivedMsg.Egress.OriginID, msg.Egress.OriginID)
				require.Equal(t, receivedMsg.Egress.TargetNum, msg.Egress.TargetNum)
				require.Equal(t, receivedMsg.Egress.TargetIDs, msg.Egress.TargetIDs)
				require.Equal(t, receivedMsg.Egress.ChannelID, msg.Egress.ChannelID)

				close(sentMsgReceived)
			})

		// goroutine checks the mock ccf for receiving the register message from connector.
		// the register message arrives as the connector attempts a connection on to the ccf.
		registerMsgReceived := make(chan struct{})
		go func() {
			<-ccf.attackerRegMsg
			close(registerMsgReceived)
		}()

		// creates a connection to ccf.
		_, err = connector.Connect(ctx, corruptedId.NodeID)
		require.NoError(t, err)

		// checks a timely attacker registration as well as arrival of the sent message by ccf to corrupted connection
		unittest.RequireCloseBefore(t, registerMsgReceived, 1*time.Second, "ccf could not receive attacker registration on time")

		// imitates sending a message from ccf to attacker through corrupted connection.
		require.NoError(t, ccf.attackerObserveStream.Send(msg))

		unittest.RequireCloseBefore(t, sentMsgReceived, 1*time.Second, "corrupted connection could not receive ccf message on time")
	})
}

// withMockCorruptibleConduitFactory creates and starts a mock corruptible conduit factory. This mock factory only runs the gRPC server part of an
// actual corruptible conduit factory, and then executes the run function on it.
func withMockCorruptibleConduitFactory(t *testing.T, run func(flow.Identity, irrecoverable.SignalerContext, *mockCorruptibleConduitFactory)) {
	corruptedIdentity := unittest.IdentityFixture(unittest.WithAddress(insecure.DefaultAddress))

	// life-cycle management of corruptible conduit factory.
	ctx, cancel := context.WithCancel(context.Background())
	ccfCtx, errChan := irrecoverable.WithSignaler(ctx)
	go func() {
		select {
		case err := <-errChan:
			t.Error("mock corruptible conduit factory startup encountered fatal error", err)
		case <-ctx.Done():
			return
		}
	}()

	ccf := newMockCorruptibleConduitFactory()

	// starts corruptible conduit factory
	ccf.Start(ccfCtx)
	unittest.RequireCloseBefore(t, ccf.Ready(), 1*time.Second, "could not start corruptible conduit factory on time")

	run(*corruptedIdentity, ccfCtx, ccf)

	// terminates attackNetwork
	cancel()
	unittest.RequireCloseBefore(t, ccf.Done(), 1*time.Second, "could not stop corruptible conduit on time")
}
