package attacknetwork

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/insecure"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/network/codec/cbor"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestConnectorHappy path checks that a CorruptedConnector can successfully create a connection to a remote corruptible conduit factory.
// Moreover, it checks that the resulted connection is capable of intact message delivery in a timely fashion.
func TestConnectorHappyPath(t *testing.T) {
	withMockCorruptibleConduitFactory(t, func(corruptedId flow.Identity, factory *mockCorruptibleConduitFactory) {
		connector := NewCorruptedConnector(flow.IdentityList{&corruptedId})
		// attacker address is solely used as part of register message,
		// hence no real network address needed.
		attackerAddress := "dummy-attacker-address"

		connector.WithAttackerAddress(attackerAddress)

		// goroutine checks the mock factory for receiving the register message from connector.
		// the register message arrives as the connector attempts a connection on to the factory.
		registerMsgReceived := make(chan struct{})
		go func() {
			receivedRegMsg := <-factory.attackerRegMsg
			// register message should contain attacker address
			require.Equal(t, attackerAddress, receivedRegMsg.Address)

			close(registerMsgReceived)
		}()

		// goroutine checks mock factory for receiving the message sent over the connection.
		msg, _, _ := messageFixture(t, cbor.NewCodec(), insecure.Protocol_MULTICAST)
		sentMsgReceived := make(chan struct{})
		go func() {
			receivedMsg := <-factory.attackerMsg

			// received message should have an exact match on the relevant fields.
			// Note: only fields filled by test fixtures are checked, as some others
			// are filled by gRPC on the fly, which are not relevant to the test's sanity.
			require.Equal(t, receivedMsg.Payload, msg.Payload)
			require.Equal(t, receivedMsg.Protocol, msg.Protocol)
			require.Equal(t, receivedMsg.OriginID, msg.OriginID)
			require.Equal(t, receivedMsg.TargetNum, msg.TargetNum)
			require.Equal(t, receivedMsg.TargetIDs, msg.TargetIDs)
			require.Equal(t, receivedMsg.ChannelID, msg.ChannelID)

			close(sentMsgReceived)
		}()

		// creates a connection to the corruptible conduit factory.
		connection, err := connector.Connect(context.Background(), corruptedId.NodeID)
		require.NoError(t, err)

		// sends a message over the corruptible conduit factory
		require.NoError(t, connection.SendMessage(msg))

		// checks a timely arrival of the registration and sent messages at the factory.
		unittest.RequireCloseBefore(t, registerMsgReceived, 1*time.Second, "factory could not receive register message on time")
		unittest.RequireCloseBefore(t, sentMsgReceived, 1*time.Second, "factory could not receive message sent on connection on time")
	})
}

// withMockCorruptibleConduitFactory creates and starts a mock corruptible conduit factory. This mock factory only runs the gRPC server part of an
// actual corruptible conduit factory, and then executes the run function on it.
func withMockCorruptibleConduitFactory(t *testing.T, run func(flow.Identity, *mockCorruptibleConduitFactory)) {
	corruptedIdentity := unittest.IdentityFixture(unittest.WithAddress("localhost:0"))

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

	ccf := newMockCorruptibleConduitFactory(fmt.Sprintf("localhost:%d", insecure.CorruptedFactoryPort))

	// starts corruptible conduit factory
	ccf.Start(ccfCtx)
	unittest.RequireCloseBefore(t, ccf.Ready(), 1*time.Second, "could not start corruptible conduit factory on time")

	run(*corruptedIdentity, ccf)

	// terminates attackNetwork
	cancel()
	unittest.RequireCloseBefore(t, ccf.Done(), 1*time.Second, "could not stop corruptible conduit on time")
}
