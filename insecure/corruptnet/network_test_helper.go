package corruptnet

// This file has helper function that are only used for testing.

import (
	"context"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	grpcinsecure "google.golang.org/grpc/credentials/insecure"

	"github.com/onflow/flow-go/insecure"
	"github.com/onflow/flow-go/model/bootstrap"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/module/local"
	"github.com/onflow/flow-go/network/mocknetwork"
	"github.com/onflow/flow-go/utils/unittest"
)

// corruptNetworkFixture creates a corruptible Network with a mock Adapter.
// By default, no attacker is registered on this corruptible network.
// This function is not meant to be used by tests directly because it expects the corrupt network to be properly started and stopped.
// Otherwise, it will throw mock expectations errors.
func corruptNetworkFixture(t *testing.T, logger zerolog.Logger, corruptedID ...flow.Identifier) (*Network, *mocknetwork.Adapter, bootstrap.NodeInfo) {
	// create corruptible network with no attacker registered
	codec := unittest.NetworkCodec()

	corruptedIdentity := unittest.PrivateNodeInfoFixture(unittest.WithAddress(insecure.DefaultAddress))
	// some tests will want to create corruptible network with a specific ID
	if len(corruptedID) > 0 {
		corruptedIdentity.NodeID = corruptedID[0]
	}

	flowNetwork := mocknetwork.NewNetwork(t)
	flowNetwork.On("Start", mock.Anything).Return()

	// mock flow network will pretend to be ready when required
	ready := make(chan struct{})
	close(ready)
	flowNetwork.On("Ready", mock.Anything).Return(func() <-chan struct{} {
		return ready
	})

	// mock flow network will pretend to be done when required
	done := make(chan struct{})
	close(done)
	flowNetwork.On("Done", mock.Anything).Return(func() <-chan struct{} {
		return done
	})
	ccf := NewCorruptConduitFactory(logger, flow.BftTestnet)

	// set up adapter, so we can check that it called the expected method.
	// It will be checked automatically without having to remember to call mock.AssertExpectationsForObjects()
	adapter := mocknetwork.NewAdapter(t)
	err := ccf.RegisterAdapter(adapter)
	require.NoError(t, err)

	private, err := corruptedIdentity.PrivateKeys()
	require.NoError(t, err)
	me, err := local.New(corruptedIdentity.Identity().IdentitySkeleton, private.StakingKey)
	require.NoError(t, err)
	corruptibleNetwork, err := NewCorruptNetwork(
		logger,
		flow.BftTestnet,
		insecure.DefaultAddress,
		me,
		codec,
		flowNetwork,
		ccf)
	require.NoError(t, err)

	// return adapter so callers can set up test specific expectations
	return corruptibleNetwork, adapter, corruptedIdentity
}

// runCorruptNetworkTest creates and starts a corruptible network, runs the "run" function of a simulated attacker and then
// terminates the network.
func runCorruptNetworkTest(t *testing.T, logger zerolog.Logger,
	run func(
		flow.Identity, // identity of ccf
		*Network, // corruptible network
		*mocknetwork.Adapter, // mock adapter that corrupted network uses to communicate with authorized flow nodes.
		insecure.CorruptNetwork_ProcessAttackerMessageClient, // gRPC interface that orchestrator network uses to send messages to this ccf.
	)) {

	// life-cycle management of corruptible network
	ctx, cancel := context.WithCancel(context.Background())
	ccfCtx, errChan := irrecoverable.WithSignaler(ctx)
	go func() {
		select {
		case err := <-errChan:
			t.Error("corruptible network startup encountered fatal error", err)
		case <-ctx.Done():
			return
		}
	}()

	corruptedIdentifier := unittest.IdentifierFixture()
	corruptibleNetwork, adapter, corruptedIdentity := corruptNetworkFixture(t, logger, corruptedIdentifier)

	// start corruptible network
	corruptibleNetwork.Start(ccfCtx)
	unittest.RequireCloseBefore(t, corruptibleNetwork.Ready(), 100*time.Millisecond, "could not start corruptible network on time")

	// extracting port that ccf gRPC server is running on
	_, ccfPortStr, err := net.SplitHostPort(corruptibleNetwork.ServerAddress())
	require.NoError(t, err)

	// imitating an attacker dial to corruptible network and opening a stream to it
	// on which the attacker dictates to relay messages on the actual flow network
	gRpcClient, err := grpc.Dial(
		fmt.Sprintf("localhost:%s", ccfPortStr),
		grpc.WithTransportCredentials(grpcinsecure.NewCredentials()))
	require.NoError(t, err)

	client := insecure.NewCorruptNetworkClient(gRpcClient)
	stream, err := client.ProcessAttackerMessage(context.Background())
	require.NoError(t, err)

	run(*corruptedIdentity.Identity(), corruptibleNetwork, adapter, stream)

	// terminates orchestratorNetwork
	cancel()
	unittest.RequireCloseBefore(t, corruptibleNetwork.Done(), 100*time.Millisecond, "could not stop corruptible conduit on time")
}
