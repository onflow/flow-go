package test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/insecure/corruptible"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/libp2p/message"
	"github.com/onflow/flow-go/module/irrecoverable"
	flownet "github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/network/codec/cbor"
	"github.com/onflow/flow-go/network/stub"
	"github.com/onflow/flow-go/utils/unittest"
	"github.com/onflow/flow-go/utils/unittest/network"
)

func TestComposability(t *testing.T) {
	withCorruptibleConduitFactory(t, func(t *testing.T, corruptedIdentity flow.Identity, ccf *corruptible.ConduitFactory) {
		hub := stub.NewNetworkHub()
		originalEvent := &message.TestMessage{Text: "this is a test message"}
		corruptedEvent := &message.TestMessage{Text: "this is a corrupted message"}
		testChannel := flownet.Channel("test-channel")

		honestIdentity := unittest.IdentityFixture()

		corruptedNodeNetwork := stub.NewNetwork(t, corruptedIdentity.NodeID, hub, stub.WithConduitFactory(ccf))
		corruptedEngine := &network.Engine{}
		corruptedConduit, err := corruptedNodeNetwork.Register(testChannel, corruptedEngine)
		require.NoError(t, err)

		honestNodeNetwork := stub.NewNetwork(t, honestIdentity.NodeID, hub)
		honestEngine := &network.Engine{}

		wg := &sync.WaitGroup{}
		wg.Add(1)
		honestEngine.OnProcess(func(channel flownet.Channel, originId flow.Identifier, event interface{}) error {
			require.Equal(t, testChannel, channel)               // event must arrive at the channel set by orchestrator.
			require.Equal(t, corruptedIdentity.NodeID, originId) // origin id of the message must be the corrupted node.
			require.NotEqual(t, corruptedEvent, event)           // content of event must be swapped with corrupted event.

			wg.Done()
			return nil
		})
		_, err = honestNodeNetwork.Register(testChannel, honestEngine)
		require.NoError(t, err)

		unittest.RequireReturnsBefore(t, func() {
			corruptedNodeNetwork.StartConDev(100*time.Millisecond, true)
		}, 100*time.Millisecond, "failed to start corrupted node network")

		require.NoError(t, corruptedConduit.Unicast(originalEvent, honestIdentity.NodeID))

		unittest.RequireReturnsBefore(t, wg.Wait, 1*time.Second, "honest node could not receive corrupted event on time")
		unittest.RequireReturnsBefore(t, func() {
			corruptedNodeNetwork.StopConDev()
		}, 100*time.Millisecond, "failed to stop verification network")
	})

}

func withCorruptibleConduitFactory(t *testing.T, run func(*testing.T, flow.Identity, *corruptible.ConduitFactory)) {
	codec := cbor.NewCodec()
	corruptedIdentity := unittest.IdentityFixture()

	// life-cycle management of attackNetwork.
	ctx, cancel := context.WithCancel(context.Background())
	ccfCtx, errChan := irrecoverable.WithSignaler(ctx)
	go func() {
		select {
		case err := <-errChan:
			t.Error("attackNetwork startup encountered fatal error", err)
		case <-ctx.Done():
			return
		}
	}()
	ccf := corruptible.NewCorruptibleConduitFactory(unittest.Logger(), flow.BftTestnet, corruptedIdentity.NodeID, codec)

	// starts corruptible conduit factory
	ccf.Start(ccfCtx)
	unittest.RequireCloseBefore(t, ccf.Ready(), 1*time.Second, "could not start corruptible conduit factory on time")

	run(t, *corruptedIdentity, ccf)

	// terminates attackNetwork
	cancel()
	unittest.RequireCloseBefore(t, ccf.Done(), 1*time.Second, "could not stop corruptible conduit on time")
}
