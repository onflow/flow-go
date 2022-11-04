package corruptible

// This file has helper function that are only used for testing.

import (
	"context"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/rs/zerolog"

	"google.golang.org/grpc"
	grpcinsecure "google.golang.org/grpc/credentials/insecure"

	"github.com/onflow/flow-go/insecure"
	"github.com/onflow/flow-go/module/irrecoverable"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/engine/testutil"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/network/mocknetwork"
	"github.com/onflow/flow-go/utils/unittest"
)

// corruptibleNetworkFixture creates a corruptible Network with a mock Adapter.
// By default, no attacker is registered on this corruptible network.
func corruptibleNetworkFixture(t *testing.T, logger zerolog.Logger, corruptedID ...*flow.Identity) (*Network, *mocknetwork.Adapter) {
	// create corruptible network with no attacker registered
	codec := unittest.NetworkCodec()

	corruptedIdentity := unittest.IdentityFixture(unittest.WithAddress(insecure.DefaultAddress))
	// some tests will want to create corruptible network with a specific ID
	if len(corruptedID) > 0 {
		corruptedIdentity = corruptedID[0]
	}

	flowNetwork := &mocknetwork.Network{}
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
	ccf := NewCorruptibleConduitFactory(logger, flow.BftTestnet)

	// set up adapter, so we can check that it called the expected method
	adapter := &mocknetwork.Adapter{}
	err := ccf.RegisterAdapter(adapter)
	require.NoError(t, err)

	corruptibleNetwork, err := NewCorruptNetwork(
		logger,
		flow.BftTestnet,
		insecure.DefaultAddress,
		testutil.LocalFixture(t, corruptedIdentity),
		codec,
		flowNetwork,
		ccf)
	require.NoError(t, err)

	// return adapter so callers can set up test specific expectations
	return corruptibleNetwork, adapter
}

// withCorruptibleNetwork creates and starts a corruptible network, runs the "run" function and then
// terminates the network.
func withCorruptibleNetwork(t *testing.T, logger zerolog.Logger,
	run func(
		flow.Identity, // identity of ccf
		*Network, // corruptible network
		*mocknetwork.Adapter, // mock adapter that corrupted network uses to communicate with authorized flow nodes.
		insecure.CorruptibleConduitFactory_ProcessAttackerMessageClient, // gRPC interface that orchestrator network uses to send messages to this ccf.
	)) {

	corruptedIdentity := unittest.IdentityFixture(unittest.WithAddress(insecure.DefaultAddress))

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

	corruptibleNetwork, adapter := corruptibleNetworkFixture(t, logger, corruptedIdentity)

	// start corruptible network
	corruptibleNetwork.Start(ccfCtx)
	unittest.RequireCloseBefore(t, corruptibleNetwork.Ready(), 1*time.Second, "could not start corruptible network on time")

	// extracting port that ccf gRPC server is running on
	_, ccfPortStr, err := net.SplitHostPort(corruptibleNetwork.ServerAddress())
	require.NoError(t, err)

	// imitating an attacker dial to corruptible network and opening a stream to it
	// on which the attacker dictates to relay messages on the actual flow network
	gRpcClient, err := grpc.Dial(
		fmt.Sprintf("localhost:%s", ccfPortStr),
		grpc.WithTransportCredentials(grpcinsecure.NewCredentials()))
	require.NoError(t, err)

	client := insecure.NewCorruptibleConduitFactoryClient(gRpcClient)
	stream, err := client.ProcessAttackerMessage(context.Background())
	require.NoError(t, err)

	run(*corruptedIdentity, corruptibleNetwork, adapter, stream)

	// terminates orchestratorNetwork
	cancel()
	unittest.RequireCloseBefore(t, corruptibleNetwork.Done(), 1*time.Second, "could not stop corruptible conduit on time")
}
