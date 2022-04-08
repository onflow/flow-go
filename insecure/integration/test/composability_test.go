package test

import (
	"context"
	"net"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/insecure"
	"github.com/onflow/flow-go/insecure/attacknetwork"
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

// TestCorruptibleConduitFrameworkHappyPath implements a round-trip test that checks the composibility of the entire happy path of the corruptible
// conduit framework. The test runs two engines on two distinct network instances, one with normal conduit,
// and one with a corruptible conduit factory (ccf).
//
// The engine running on ccf is sending a message to the other engine (running with normal conduit).
// The message goes through ccf to the attack network and reaches the attack orchestrator.
// The orchestrator corrupts the message and sends it back to the ccf to be dispatched on the flow network.
// The test passes if the engine running with normal conduit receives the corrupted message in a timely fashion (and also never gets the
// original messages).
func TestCorruptibleConduitFrameworkHappyPath(t *testing.T) {
	// We first start ccf and then the attack network, since the order of startup matters, i.e., on startup, the attack network tries
	// to connect to all ccfs.
	withCorruptibleConduitFactory(t, func(t *testing.T, corruptedIdentity flow.Identity, ccf *corruptible.ConduitFactory) {
		// this is the event orchestrator will send instead of the original event coming from corrupted engine.
		corruptedEvent := &message.TestMessage{Text: "this is a corrupted message"}

		// extracting port that ccf gRPC server is running on
		_, ccfPortStr, err := net.SplitHostPort(ccf.ServerAddress())
		require.NoError(t, err)
		ccfPort, err := strconv.Atoi(ccfPortStr)
		require.NoError(t, err)

		withAttackOrchestrator(t, flow.IdentityList{&corruptedIdentity}, ccfPort, func(event *insecure.Event) {
			// implementing the corruption functionality of the orchestrator.
			event.FlowProtocolEvent = corruptedEvent
		}, func(t *testing.T) {
			hub := stub.NewNetworkHub()
			originalEvent := &message.TestMessage{Text: "this is a test message"}
			testChannel := flownet.Channel("test-channel")

			// corrupted node network
			corruptedEngine := &network.Engine{}
			corruptedNodeNetwork := stub.NewNetwork(t, corruptedIdentity.NodeID, hub, stub.WithConduitFactory(ccf))
			corruptedConduit, err := corruptedNodeNetwork.Register(testChannel, corruptedEngine)
			require.NoError(t, err)

			// honest network
			honestIdentity := unittest.IdentityFixture()
			honestEngine := &network.Engine{}
			honestNodeNetwork := stub.NewNetwork(t, honestIdentity.NodeID, hub)
			// in this test, the honest node is only the receiver, hence, we discard
			// the created conduit.
			_, err = honestNodeNetwork.Register(testChannel, honestEngine)
			require.NoError(t, err)

			wg := &sync.WaitGroup{}
			wg.Add(1)
			honestEngine.OnProcess(func(channel flownet.Channel, originId flow.Identifier, event interface{}) error {
				// implementing the process logic of the honest engine on reception of message from underlying network.
				require.Equal(t, testChannel, channel)               // event must arrive at the channel set by orchestrator.
				require.Equal(t, corruptedIdentity.NodeID, originId) // origin id of the message must be the corrupted node.
				require.Equal(t, corruptedEvent, event)              // content of event must be swapped with corrupted event.

				wg.Done()
				return nil
			})

			unittest.RequireReturnsBefore(t, func() {
				// starts the stub network of the corrupted node so that messages sent by its registered engines can be delivered.
				corruptedNodeNetwork.StartConDev(100*time.Millisecond, true)
			}, 100*time.Millisecond, "failed to start corrupted node network")

			require.NoError(t, corruptedConduit.Unicast(originalEvent, honestIdentity.NodeID))

			unittest.RequireReturnsBefore(t, wg.Wait, 1*time.Second, "honest node could not receive corrupted event on time")
			unittest.RequireReturnsBefore(t, func() {
				// stops the stub network of corrupted node.
				corruptedNodeNetwork.StopConDev()
			}, 100*time.Millisecond, "failed to stop verification network")
		})
	})

}

// withCorruptibleConduitFactory creates a real corruptible conduit factory (ccf), starts it, runs the "run" function, and then stops it.
func withCorruptibleConduitFactory(t *testing.T, run func(*testing.T, flow.Identity, *corruptible.ConduitFactory)) {
	codec := cbor.NewCodec()
	corruptedIdentity := unittest.IdentityFixture(unittest.WithAddress("localhost:0"))

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
	ccf := corruptible.NewCorruptibleConduitFactory(
		unittest.Logger(),
		flow.BftTestnet,
		corruptedIdentity.NodeID,
		codec,
		"localhost:0")

	// starts corruptible conduit factory
	ccf.Start(ccfCtx)
	unittest.RequireCloseBefore(
		t,
		ccf.Ready(),
		1*time.Second,
		"could not start corruptible conduit factory on time")

	run(t, *corruptedIdentity, ccf)

	// terminates attackNetwork
	cancel()
	unittest.RequireCloseBefore(t, ccf.Done(), 1*time.Second, "could not stop corruptible conduit on time")
}

// withAttackOrchestrator creates a mock orchestrator with the injected "corrupter" function, which entirely runs on top of a real attack network.
// It then starts the attack network, executes the "run" function, and stops the attack network afterwards.
func withAttackOrchestrator(t *testing.T, corruptedIds flow.IdentityList, ccfPort int, corrupter func(*insecure.Event), run func(t *testing.T)) {
	codec := cbor.NewCodec()
	o := &mockOrchestrator{eventCorrupter: corrupter}
	connector := attacknetwork.NewCorruptedConnector(corruptedIds, ccfPort)
	attackNetwork, err := attacknetwork.NewAttackNetwork(unittest.Logger(), "localhost:0", codec, o, connector, corruptedIds)
	require.NoError(t, err)

	// life-cycle management of attackNetwork.
	ctx, cancel := context.WithCancel(context.Background())
	attackNetworkCtx, errChan := irrecoverable.WithSignaler(ctx)
	go func() {
		select {
		case err := <-errChan:
			t.Error("attackNetwork startup encountered fatal error", err)
		case <-ctx.Done():
			return
		}
	}()

	// starts corruptible conduit factory
	attackNetwork.Start(attackNetworkCtx)
	unittest.RequireCloseBefore(t, attackNetwork.Ready(), 1*time.Second, "could not start attack network on time")
	run(t)

	// terminates attackNetwork
	cancel()
	unittest.RequireCloseBefore(t, attackNetwork.Done(), 1*time.Second, "could not stop attack network on time")
}
