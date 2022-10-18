package test

import (
	"context"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"

	"github.com/onflow/flow-go/network/mocknetwork"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/engine/testutil"
	"github.com/onflow/flow-go/insecure"
	"github.com/onflow/flow-go/insecure/attackernet"
	"github.com/onflow/flow-go/insecure/corruptnet"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/libp2p/message"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/network/channels"
	"github.com/onflow/flow-go/network/stub"
	"github.com/onflow/flow-go/utils/unittest"
)

type Channel string

// TestCorruptNetworkFrameworkHappyPath implements a round-trip test that checks the composibility of the entire happy path of the corrupt
// network framework. The test runs two engines on two distinct network instances, one with normal conduit,
// and one with a corrupt conduit factory (ccf).
//
// The engine running on corrupt network is sending a message to the honest engine.
// The message goes through the corrupt network to the orchestrator network and reaches the attack orchestrator.
// The orchestrator corrupts the message and sends it back to the corrupt network to be dispatched on the flow network.
// The test passes if the honest engine receives the corrupt message in a timely fashion (and also never gets the
// original messages).
func TestCorruptNetworkFrameworkHappyPath(t *testing.T) {
	// We first start ccf and then the orchestrator network, since the order of startup matters, i.e., on startup, the orchestrator network tries
	// to connect to all ccfs.
	withCorruptNetwork(t, func(t *testing.T, corruptIdentity flow.Identity, corruptNetwork *corruptnet.Network, hub *stub.Hub) {
		// these are the events which orchestrator will send instead of the original ingress and egress events coming to and from
		// the corrupt engine, respectively.
		corruptEgressEvent := &message.TestMessage{Text: "this is a corrupt egress message"}
		corruptIngressEvent := &message.TestMessage{Text: "this is a corrupt ingress message"}

		// extracting port that ccf gRPC server is running on
		_, ccfPortStr, err := net.SplitHostPort(corruptNetwork.ServerAddress())
		require.NoError(t, err)

		withAttackOrchestrator(t, flow.IdentityList{&corruptIdentity}, map[flow.Identifier]string{corruptIdentity.NodeID: ccfPortStr},
			func(event *insecure.EgressEvent) {
				// implementing the corruption functionality of the orchestrator for the egress traffic.
				event.FlowProtocolEvent = corruptEgressEvent
			},
			func(event *insecure.IngressEvent) {
				// implementing the corruption functionality of the orchestrator for the ingress traffic.
				event.FlowProtocolEvent = corruptIngressEvent
			},
			func(t *testing.T) {
				require.Eventually(t, func() bool {
					return corruptNetwork.AttackerRegistered() // attacker's registration must be done on corrupt network prior to sending any messages.
				}, 2*time.Second, 100*time.Millisecond, "registration of attacker on CCF could not be done one time")

				// egress event is sent from the node running corrupt engine to node running the honest engine.
				originalEgressEvent := &message.TestMessage{Text: "this is a test egress message"}
				// ingress event is sent from the node running the honest engine to node running the corrupt engine.
				originalIngressEvent := &message.TestMessage{Text: "this is a test ingress message"}
				testChannel := channels.TestNetworkChannel

				// corrupt node network
				corruptEngine := mocknetwork.NewEngine(t)
				corruptConduit, err := corruptNetwork.Register(testChannel, corruptEngine)
				require.NoError(t, err)

				// honest network
				honestIdentity := unittest.IdentityFixture()
				honestEngine := mocknetwork.NewEngine(t)
				honestNodeNetwork := stub.NewNetwork(t, honestIdentity.NodeID, hub)
				// in this test, the honest node is only the receiver, hence, we discard
				// the created conduit.
				honestNodeConduit, err := honestNodeNetwork.Register(testChannel, honestEngine)
				require.NoError(t, err)

				wg := &sync.WaitGroup{}
				wg.Add(2) // wait for both egress and ingress events to be received.

				// we expect to receive the corrupt egress event on the honest node.
				// event must arrive at the channel set by orchestrator.
				// origin id of the message must be the corrupt node.
				// content of event must be swapped with corrupt event.
				honestEngine.On("Process", testChannel, corruptIdentity.NodeID, corruptEgressEvent).Return(nil).Run(func(args mock.Arguments) {
					wg.Done()
				})

				// we expect to receive the corrupt ingress event on the corrupt node.
				// event must arrive at the channel set by orchestrator.
				// origin id of the message must be the honest node.
				// content of event must be swapped with corrupt event.
				corruptEngine.On("Process", testChannel, honestIdentity.NodeID, corruptIngressEvent).Return(nil).Run(func(args mock.Arguments) {
					// simulate the Process logic of the corrupt engine on reception of message from underlying network.
					wg.Done()
				})

				go func() {
					// sending egress event from corrupt node to honest node.
					require.NoError(t, corruptConduit.Unicast(originalEgressEvent, honestIdentity.NodeID))
				}()
				go func() {
					// sending ingress event from honest node to corrupt node.
					require.NoError(t, honestNodeConduit.Unicast(originalIngressEvent, corruptIdentity.NodeID))
				}()

				// wait for both egress and ingress events to be received.
				unittest.RequireReturnsBefore(t, wg.Wait, 1*time.Second, "honest node could not receive corrupt event on time")
			})
	})

}

// withCorruptNetwork creates a real corrupt network, starts it, runs the "run" function, and then stops it.
func withCorruptNetwork(t *testing.T, run func(*testing.T, flow.Identity, *corruptnet.Network, *stub.Hub)) {
	codec := unittest.NetworkCodec()
	corruptIdentity := unittest.IdentityFixture(unittest.WithAddress(insecure.DefaultAddress))

	// life-cycle management of orchestratorNetwork.
	ctx, cancel := context.WithCancel(context.Background())
	ccfCtx, errChan := irrecoverable.WithSignaler(ctx)
	go func() {
		select {
		case err := <-errChan:
			t.Error("orchestratorNetwork startup encountered fatal error", err)
		case <-ctx.Done():
			return
		}
	}()
	hub := stub.NewNetworkHub()
	ccf := corruptnet.NewCorruptConduitFactory(unittest.Logger(), flow.BftTestnet)
	flowNetwork := stub.NewNetwork(t, corruptIdentity.NodeID, hub, stub.WithConduitFactory(ccf))
	corruptNetwork, err := corruptnet.NewCorruptNetwork(
		unittest.Logger(),
		flow.BftTestnet,
		insecure.DefaultAddress,
		testutil.LocalFixture(t, corruptIdentity),
		codec,
		flowNetwork,
		ccf)
	require.NoError(t, err)

	// starts corrupt network
	corruptNetwork.Start(ccfCtx)
	unittest.RequireCloseBefore(
		t,
		corruptNetwork.Ready(),
		100*time.Millisecond,
		"could not start corrupt network on time")

	unittest.RequireReturnsBefore(t, func() {
		// starts the stub network of the corrupt node so that messages sent by its registered engines can be delivered.
		flowNetwork.StartConDev(100*time.Millisecond, true)
	}, 100*time.Millisecond, "failed to start corrupt node network")

	run(t, *corruptIdentity, corruptNetwork, hub)

	// terminates orchestratorNetwork
	cancel()
	unittest.RequireCloseBefore(t, corruptNetwork.Done(), 100*time.Millisecond, "could not stop corrupt network on time")

	unittest.RequireReturnsBefore(t, func() {
		// stops the stub network of corrupt node.
		flowNetwork.StopConDev()
	}, 100*time.Millisecond, "failed to stop verification network")
}

// withAttackOrchestrator creates a mock orchestrator with the injected "corrupter" function, which entirely runs on top of a real orchestrator network.
// It then starts the orchestrator network, executes the "run" function, and stops the orchestrator network afterwards.
func withAttackOrchestrator(
	t *testing.T,
	corruptIds flow.IdentityList,
	corruptPortMap map[flow.Identifier]string,
	egressEventCorrupter func(*insecure.EgressEvent),
	ingressEventCorrupter func(*insecure.IngressEvent),
	run func(t *testing.T)) {
	codec := unittest.NetworkCodec()
	o := &mockOrchestrator{
		egressEventCorrupter:  egressEventCorrupter,
		ingressEventCorrupter: ingressEventCorrupter,
	}
	connector := attackernet.NewCorruptConnector(unittest.Logger(), corruptIds, corruptPortMap)

	orchestratorNetwork, err := attackernet.NewOrchestratorNetwork(
		unittest.Logger(),
		codec,
		o,
		connector,
		corruptIds)
	require.NoError(t, err)

	// life-cycle management of orchestratorNetwork.
	ctx, cancel := context.WithCancel(context.Background())
	orchestratorNetworkCtx, errChan := irrecoverable.WithSignaler(ctx)
	go func() {
		select {
		case err := <-errChan:
			t.Error("orchestratorNetwork startup encountered fatal error", err)
		case <-ctx.Done():
			return
		}
	}()

	// starts orchestrator network
	orchestratorNetwork.Start(orchestratorNetworkCtx)
	unittest.RequireCloseBefore(t, orchestratorNetwork.Ready(), 100*time.Millisecond, "could not start orchestrator network on time")
	run(t)

	// terminates orchestratorNetwork
	cancel()
	unittest.RequireCloseBefore(t, orchestratorNetwork.Done(), 100*time.Millisecond, "could not stop orchestrator network on time")
}
