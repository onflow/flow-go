package orchestrator

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

// TestConnectorHappy_Send checks that a CorruptedConnector can successfully create a connection to a remote corrupt network (CN).
// Moreover, it checks that the resulted connection is capable of intact message delivery in a timely fashion from attacker to corrupt network.
func TestConnectorHappyPath_Send(t *testing.T) {
	withMockCorruptNetwork(t, func(corruptedId flow.Identity, ctx irrecoverable.SignalerContext, cn *mockCorruptNetwork) {
		// extracting port that CN gRPC server is running on.
		_, cnPortStr, err := net.SplitHostPort(cn.ServerAddress())
		require.NoError(t, err)

		connector := NewCorruptedConnector(unittest.Logger(),
			flow.IdentityList{&corruptedId},
			map[flow.Identifier]string{corruptedId.NodeID: cnPortStr})
		// empty incoming handler, as this test does not evaluate receive path
		connector.WithIncomingMessageHandler(func(i *insecure.Message) {})

		// goroutine checks the mock cn for receiving the attacker registration from connector.
		// the attacker registration arrives as the connector attempts a connection on to the cn.
		attackerRegistered := make(chan struct{})
		go func() {
			<-cn.attackerRegMsg
			close(attackerRegistered)
		}()

		// goroutine checks mock cn for receiving the message sent over the connection.
		msg, _, _ := insecure.EgressMessageFixture(t, unittest.NetworkCodec(), insecure.Protocol_MULTICAST, &message.TestMessage{
			Text: fmt.Sprintf("this is a test message from attacker to cn: %d", rand.Int()),
		})
		sentMsgReceived := make(chan struct{})
		go func() {
			receivedMsg := <-cn.attackerMsg

			// received message should have an exact match on the relevant fields.
			// Note: only fields filled by test fixtures are checked, as some others
			// are filled by gRPC on the fly, which are not relevant to the test's sanity.
			require.Equal(t, receivedMsg.Egress.Payload, msg.Egress.Payload)
			require.Equal(t, receivedMsg.Egress.Protocol, msg.Egress.Protocol)
			require.Equal(t, receivedMsg.Egress.CorruptOriginID, msg.Egress.CorruptOriginID)
			require.Equal(t, receivedMsg.Egress.TargetNum, msg.Egress.TargetNum)
			require.Equal(t, receivedMsg.Egress.TargetIDs, msg.Egress.TargetIDs)
			require.Equal(t, receivedMsg.Egress.ChannelID, msg.Egress.ChannelID)

			close(sentMsgReceived)
		}()

		// creates a connection to the corrupt network.
		connection, err := connector.Connect(ctx, corruptedId.NodeID)
		require.NoError(t, err)
		defer func() {
			require.NoError(t, connection.CloseConnection())
		}()

		// sends a message to cn
		require.NoError(t, connection.SendMessage(msg))

		// checks a timely arrival of the registration and sent message at the cn.
		unittest.RequireCloseBefore(t, attackerRegistered, 100*time.Millisecond, "cn could not receive attacker registration on time")
		// imitates sending a message from cn to attacker through corrupted connection.
		unittest.RequireCloseBefore(t, sentMsgReceived, 100*time.Millisecond, "cn could not receive message sent on connection on time")
	})
}

// TestConnectorHappy_Receive checks that a CorruptedConnector can successfully create a connection to a remote corrupt network (CN).
// Moreover, it checks that the resulted connection is capable of intact message delivery in a timely fashion from CN to attacker.
func TestConnectorHappyPath_Receive(t *testing.T) {
	withMockCorruptNetwork(t, func(corruptedId flow.Identity, ctx irrecoverable.SignalerContext, cn *mockCorruptNetwork) {
		// extracting port that CN gRPC server is running on
		_, cnPortStr, err := net.SplitHostPort(cn.ServerAddress())
		require.NoError(t, err)

		msg, _, _ := insecure.EgressMessageFixture(t, unittest.NetworkCodec(), insecure.Protocol_MULTICAST, &message.TestMessage{
			Text: fmt.Sprintf("this is a test message from cn to attacker: %d", rand.Int()),
		})

		sentMsgReceived := make(chan struct{})
		connector := NewCorruptedConnector(unittest.Logger(),
			flow.IdentityList{&corruptedId},
			map[flow.Identifier]string{corruptedId.NodeID: cnPortStr})
		connector.WithIncomingMessageHandler(
			func(receivedMsg *insecure.Message) {
				// received message by attacker should have an exact match on the relevant fields as sent by cn.
				// Note: only fields filled by test fixtures are checked, as some others
				// are filled by gRPC on the fly, which are not relevant to the test's sanity.
				require.Equal(t, receivedMsg.Egress.Payload, msg.Egress.Payload)
				require.Equal(t, receivedMsg.Egress.Protocol, msg.Egress.Protocol)
				require.Equal(t, receivedMsg.Egress.CorruptOriginID, msg.Egress.CorruptOriginID)
				require.Equal(t, receivedMsg.Egress.TargetNum, msg.Egress.TargetNum)
				require.Equal(t, receivedMsg.Egress.TargetIDs, msg.Egress.TargetIDs)
				require.Equal(t, receivedMsg.Egress.ChannelID, msg.Egress.ChannelID)

				close(sentMsgReceived)
			})

		// goroutine checks the mock cn for receiving the register message from connector.
		// the register message arrives as the connector attempts a connection on to the cn.
		registerMsgReceived := make(chan struct{})
		go func() {
			<-cn.attackerRegMsg
			close(registerMsgReceived)
		}()

		// creates a connection to cn.
		_, err = connector.Connect(ctx, corruptedId.NodeID)
		require.NoError(t, err)

		// checks a timely attacker registration as well as arrival of the sent message by cn to corrupted connection
		unittest.RequireCloseBefore(t, registerMsgReceived, 100*time.Millisecond, "cn could not receive attacker registration on time")

		// imitates sending a message from cn to attacker through corrupted connection.
		require.NoError(t, cn.attackerObserveStream.Send(msg))

		unittest.RequireCloseBefore(t, sentMsgReceived, 100*time.Millisecond, "corrupted connection could not receive cn message on time")
	})
}

// withMockCorruptNetwork creates and starts a mock corrupt network. This mock corrupt network only runs the gRPC server part of an
// actual corrupt network, and then executes the run function on it.
func withMockCorruptNetwork(t *testing.T, run func(flow.Identity, irrecoverable.SignalerContext, *mockCorruptNetwork)) {
	corruptedIdentity := unittest.IdentityFixture(unittest.WithAddress(insecure.DefaultAddress))

	// life-cycle management of corrupt network.
	ctx, cancel := context.WithCancel(context.Background())
	cnCtx, errChan := irrecoverable.WithSignaler(ctx)
	go func() {
		select {
		case err := <-errChan:
			t.Error("mock corrupt network startup encountered fatal error", err)
		case <-ctx.Done():
			return
		}
	}()

	cn := newMockCorruptNetwork()

	// starts corrupt network
	cn.Start(cnCtx)
	unittest.RequireCloseBefore(t, cn.Ready(), 100*time.Millisecond, "could not start corrupt network on time")

	run(*corruptedIdentity, cnCtx, cn)

	// terminates orchestratorNetwork
	cancel()
	unittest.RequireCloseBefore(t, cn.Done(), 100*time.Millisecond, "could not stop corrupt network on time")
}
