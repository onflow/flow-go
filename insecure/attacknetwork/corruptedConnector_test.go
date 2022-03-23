package attacknetwork

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/insecure"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/network/codec/cbor"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestConnectorHappyPath(t *testing.T) {
	withMockCorruptibleConduitFactory(t, func(corruptedId flow.Identity, factory *mockCorruptibleConduitFactory) {
		connector := NewCorruptedConnector(flow.IdentityList{&corruptedId})
		attackerAddress := "dummy-attacker-address"
		connector.WithAttackerAddress(attackerAddress)

		registerMsgReceived := make(chan struct{})
		go func() {
			receivedRegMsg := <-factory.attackerRegMsg
			require.Equal(t, attackerAddress, receivedRegMsg.Address)

			close(registerMsgReceived)
		}()

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

		connection, err := connector.Connect(context.Background(), corruptedId.NodeID)
		require.NoError(t, err)

		require.NoError(t, connection.SendMessage(msg))

		unittest.RequireCloseBefore(t, registerMsgReceived, 1*time.Second, "factory could not receive register message on time")
		unittest.RequireCloseBefore(t, sentMsgReceived, 1*time.Second, "factory could not receive message sent on connection on time")
	})
}

func withMockCorruptibleConduitFactory(t *testing.T, run func(flow.Identity, *mockCorruptibleConduitFactory)) {
	corruptedIdentity := unittest.IdentityFixture(unittest.WithAddress("localhost:0"))

	// life-cycle management of attackNetwork.
	ctx, cancel := context.WithCancel(context.Background())
	ccfCtx, errChan := irrecoverable.WithSignaler(ctx)
	go func() {
		select {
		case err := <-errChan:
			t.Error("attack network startup encountered fatal error", err)
		case <-ctx.Done():
			return
		}
	}()

	ccf := newMockCorruptibleConduitFactory("localhost:5000")

	// starts corruptible conduit factory
	ccf.Start(ccfCtx)
	unittest.RequireCloseBefore(t, ccf.Ready(), 1*time.Second, "could not start corruptible conduit factory on time")

	run(*corruptedIdentity, ccf)

	// terminates attackNetwork
	cancel()
	unittest.RequireCloseBefore(t, ccf.Done(), 1*time.Second, "could not stop corruptible conduit on time")
}
