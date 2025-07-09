package tests

import (
	"context"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"

	"github.com/onflow/flow-go/network/mocknetwork"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/insecure"
	"github.com/onflow/flow-go/insecure/corruptnet"
	"github.com/onflow/flow-go/insecure/orchestrator"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/libp2p/message"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/module/local"
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
// The test passes if the honest engine receives the corrupted message in a timely fashion (and also never gets the
// original messages).
func TestCorruptNetworkFrameworkHappyPath(t *testing.T) {
	// We first start ccf and then the orchestrator network, since the order of startup matters, i.e., on startup, the orchestrator network tries
	// to connect to all ccfs.
	withCorruptNetwork(t, func(t *testing.T, corruptedIdentity flow.Identity, corruptNetwork *corruptnet.Network, hub *stub.Hub) {
		// these are the events which orchestrator will send instead of the original ingress and egress events coming to and from
		// the corrupted engine, respectively.
		corruptedEgressEvent := &message.TestMessage{Text: "this is a corrupted egress message"}
		corruptedIngressEvent := &message.TestMessage{Text: "this is a corrupted ingress message"}

		// extracting port that ccf gRPC server is running on
		_, ccfPortStr, err := net.SplitHostPort(corruptNetwork.ServerAddress())
		require.NoError(t, err)

		withAttackOrchestrator(t, flow.IdentityList{&corruptedIdentity}, map[flow.Identifier]string{corruptedIdentity.NodeID: ccfPortStr},
			func(event *insecure.EgressEvent) {
				// implementing the corruption functionality of the orchestrator for the egress traffic.
				event.FlowProtocolEvent = corruptedEgressEvent
			},
			func(event *insecure.IngressEvent) {
				// implementing the corruption functionality of the orchestrator for the ingress traffic.
				event.FlowProtocolEvent = corruptedIngressEvent
			},
			func(t *testing.T) {
				require.Eventually(t, func() bool {
					return corruptNetwork.AttackerRegistered() // attacker's registration must be done on corrupt network prior to sending any messages.
				}, 2*time.Second, 100*time.Millisecond, "registration of attacker on CCF could not be done one time")

				// egress event is sent from the node running corrupted engine to node running the honest engine.
				originalEgressEvent := &message.TestMessage{Text: "this is a test egress message"}
				// ingress event is sent from the node running the honest engine to node running the corrupted engine.
				originalIngressEvent := &message.TestMessage{Text: "this is a test ingress message"}
				testChannel := channels.TestNetworkChannel

				// corrupted node network
				corruptedEngine := mocknetwork.NewEngine(t)
				corruptedConduit, err := corruptNetwork.Register(testChannel, corruptedEngine)
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

				// we expect to receive the corrupted egress event on the honest node.
				// event must arrive at the channel set by orchestrator.
				// origin id of the message must be the corrupted node.
				// content of event must be swapped with corrupted event.
				honestEngine.On("Process", testChannel, corruptedIdentity.NodeID, corruptedEgressEvent).Return(nil).Run(func(args mock.Arguments) {
					wg.Done()
				})

				// we expect to receive the corrupted ingress event on the corrupted node.
				// event must arrive at the channel set by orchestrator.
				// origin id of the message must be the honest node.
				// content of event must be swapped with corrupted event.
				corruptedEngine.On("Process", testChannel, honestIdentity.NodeID, corruptedIngressEvent).Return(nil).Run(func(args mock.Arguments) {
					// simulate the Process logic of the corrupted engine on reception of message from underlying network.
					wg.Done()
				})

				go func() {
					// sending egress event from corrupted node to honest node.
					require.NoError(t, corruptedConduit.Unicast(originalEgressEvent, honestIdentity.NodeID))
				}()
				go func() {
					// sending ingress event from honest node to corrupted node.
					require.NoError(t, honestNodeConduit.Unicast(originalIngressEvent, corruptedIdentity.NodeID))
				}()

				// wait for both egress and ingress events to be received.
				unittest.RequireReturnsBefore(t, wg.Wait, 1*time.Second, "honest node could not receive corrupted event on time")
			})
	})

}

// withCorruptNetwork creates a real corrupt network, starts it, runs the "run" function, and then stops it.
func withCorruptNetwork(t *testing.T, run func(*testing.T, flow.Identity, *corruptnet.Network, *stub.Hub)) {
	codec := unittest.NetworkCodec()
	corruptedIdentity := unittest.PrivateNodeInfoFixture(unittest.WithAddress(insecure.DefaultAddress))

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
	flowNetwork := stub.NewNetwork(t, corruptedIdentity.NodeID, hub, stub.WithConduitFactory(ccf))

	privateKeys, err := corruptedIdentity.PrivateKeys()
	require.NoError(t, err)
	me, err := local.New(corruptedIdentity.Identity().IdentitySkeleton, privateKeys.StakingKey)
	require.NoError(t, err)
	corruptNetwork, err := corruptnet.NewCorruptNetwork(
		unittest.Logger(),
		flow.BftTestnet,
		insecure.DefaultAddress,
		me,
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
		// starts the stub network of the corrupted node so that messages sent by its registered engines can be delivered.
		flowNetwork.StartConDev(100*time.Millisecond, true)
	}, 100*time.Millisecond, "failed to start corrupted node network")

	run(t, *corruptedIdentity.Identity(), corruptNetwork, hub)

	// terminates orchestratorNetwork
	cancel()
	unittest.RequireCloseBefore(t, corruptNetwork.Done(), 100*time.Millisecond, "could not stop corrupt network on time")

	unittest.RequireReturnsBefore(t, func() {
		// stops the stub network of corrupted node.
		flowNetwork.StopConDev()
	}, 100*time.Millisecond, "failed to stop verification network")
}

// withAttackOrchestrator creates a mock orchestrator with the injected "corrupter" function, which entirely runs on top of a real orchestrator network.
// It then starts the orchestrator network, executes the "run" function, and stops the orchestrator network afterwards.
func withAttackOrchestrator(
	t *testing.T,
	corruptedIds flow.IdentityList,
	corruptedPortMap map[flow.Identifier]string,
	egressEventCorrupter func(*insecure.EgressEvent),
	ingressEventCorrupter func(*insecure.IngressEvent),
	run func(t *testing.T)) {
	codec := unittest.NetworkCodec()
	o := &mockOrchestrator{
		egressEventCorrupter:  egressEventCorrupter,
		ingressEventCorrupter: ingressEventCorrupter,
	}
	connector := orchestrator.NewCorruptedConnector(unittest.Logger(), corruptedIds, corruptedPortMap)

	orchestratorNetwork, err := orchestrator.NewOrchestratorNetwork(
		unittest.Logger(),
		codec,
		o,
		connector,
		corruptedIds)
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
