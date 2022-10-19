package corruptnet

// This file has helper function that are only used for testing.

import (
	"context"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"

	"github.com/rs/zerolog"

	"google.golang.org/grpc"
	grpcinsecure "google.golang.org/grpc/credentials/insecure"

	"github.com/onflow/flow-go/insecure"
	"github.com/onflow/flow-go/module/irrecoverable"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/engine/testutil"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/network/mocknetwork"
	"github.com/onflow/flow-go/utils/unittest"
)

// corruptNetworkFixture creates a corrupt network with a mock Adapter.
// By default, no attacker is registered on this corrupt network.
// This function is not meant to be used by tests directly because it expects the corrupt network to be properly started and stopped.
// Otherwise, it will throw mock expectations errors.
func corruptNetworkFixture(t *testing.T, logger zerolog.Logger, corruptID ...*flow.Identity) (*Network, *mocknetwork.Adapter) {
	// create corrupt network with no attacker registered
	codec := unittest.NetworkCodec()

	corruptIdentity := unittest.IdentityFixture(unittest.WithAddress(insecure.DefaultAddress))
	// some tests will want to create corrupt network with a specific ID
	if len(corruptID) > 0 {
		corruptIdentity = corruptID[0]
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

	corruptNetwork, err := NewCorruptNetwork(
		logger,
		flow.BftTestnet,
		insecure.DefaultAddress,
		testutil.LocalFixture(t, corruptIdentity),
		codec,
		flowNetwork,
		ccf)
	require.NoError(t, err)

	// return adapter so callers can set up test specific expectations
	return corruptNetwork, adapter
}

// runCorruptNetworkTest creates and starts a corrupt network, runs the "run" function of a simulated attacker and then
// terminates the network.
func runCorruptNetworkTest(t *testing.T, logger zerolog.Logger,
	run func(
		flow.Identity, // identity of ccf
		*Network, // corrupt network
		*mocknetwork.Adapter, // mock adapter that corrupt network uses to communicate with authorized flow nodes.
		insecure.CorruptNetwork_ProcessAttackerMessageClient, // gRPC client interface that attacker network uses to send messages to this corrupt network.
	)) {

	corruptIdentity := unittest.IdentityFixture(unittest.WithAddress(insecure.DefaultAddress))

	// life-cycle management of corrupt network
	ctx, cancel := context.WithCancel(context.Background())
	ccfCtx, errChan := irrecoverable.WithSignaler(ctx)
	go func() {
		select {
		case err := <-errChan:
			t.Error("corrupt network startup encountered fatal error", err)
		case <-ctx.Done():
			return
		}
	}()

	corruptNetwork, adapter := corruptNetworkFixture(t, logger, corruptIdentity)

	// start corrupt network
	corruptNetwork.Start(ccfCtx)
	unittest.RequireCloseBefore(t, corruptNetwork.Ready(), 100*time.Millisecond, "could not start corrupt network on time")

	// extracting port that ccf gRPC server is running on
	_, ccfPortStr, err := net.SplitHostPort(corruptNetwork.ServerAddress())
	require.NoError(t, err)

	// imitating an attacker dial to corrupt network and opening a stream to it
	// on which the attacker dictates to relay messages on the actual flow network
	gRpcClient, err := grpc.Dial(
		fmt.Sprintf("localhost:%s", ccfPortStr),
		grpc.WithTransportCredentials(grpcinsecure.NewCredentials()))
	require.NoError(t, err)

	client := insecure.NewCorruptNetworkClient(gRpcClient)
	stream, err := client.ProcessAttackerMessage(context.Background())
	require.NoError(t, err)

	run(*corruptIdentity, corruptNetwork, adapter, stream)

	// terminates orchestratorNetwork
	cancel()
	unittest.RequireCloseBefore(t, corruptNetwork.Done(), 100*time.Millisecond, "could not stop corrupt conduit on time")
}
