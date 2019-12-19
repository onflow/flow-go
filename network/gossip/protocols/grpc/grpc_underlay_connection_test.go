package protocols

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/assert"

	"github.com/dapperlabs/flow-go/network/gossip"
)

// TestGRPCUnderlayConnection_Send tests if a message can be sent using the GRPC underlay
func TestGRPCUnderlayConnection_Send(t *testing.T) {
	t.Skipf(" Skipping this test since the underlay functionality is no longer needed")
	var underlay gossip.Underlay = &GRPCUnderlay{}
	address := ":0"
	listener, err := net.Listen("tcp4", address)
	require.NoError(t, err)
	// Start the Server
	go func() {
		require.NoError(t, underlay.StartWithListener(listener))
	}()
	// Setup the call back function
	ch := make(chan []byte)
	callbackfunc := func(sender string, msg []byte) {
		ch <- msg
	}
	assert.NoError(t, underlay.Handle(callbackfunc))
	// Stop Server at the end
	defer underlay.Stop()
	// Start the Client
	clientConnection, err := underlay.Dial(listener.Addr().String())
	assert.NotNil(t, clientConnection)
	assert.NoError(t, err)
	message := "hello from client"
	assert.NoError(t, clientConnection.Send(context.Background(), []byte(message)))
	recvdMessage := <-ch
	assert.Equal(t, message, string(recvdMessage))
	// Stop Client
	assert.NoError(t, clientConnection.Close())
}

// TestGRPCUnderlayConnection_OnClosed tests if the onclosed callback gets called
func TestGRPCUnderlayConnection_OnClosed(t *testing.T) {
	t.Skipf(" Skipping this test since the underlay functionality is no longer needed")
	var underlay gossip.Underlay = &GRPCUnderlay{}
	// Setup the server call back function
	ch := make(chan []byte)
	callbackfunc := func(sender string, msg []byte) {
		ch <- msg
	}
	assert.NoError(t, underlay.Handle(callbackfunc))
	address := ":0"
	listener, err := net.Listen("tcp4", address)
	require.NoError(t, err)
	// Start the Server
	go func() {
		require.NoError(t, underlay.StartWithListener(listener))
	}()

	// Stop Server at the end
	defer underlay.Stop()
	// Start the Client
	clientConnection, err := underlay.Dial(listener.Addr().String())
	assert.NotNil(t, clientConnection)
	assert.NoError(t, err)
	// Setup the client OnClose function
	closeCh := make(chan bool)
	assert.NoError(t, clientConnection.OnClosed(func() {
		closeCh <- true
	}))
	message := "hello from client"
	assert.NoError(t, clientConnection.Send(context.Background(), []byte(message)))
	select {
	case <-ch:
	case <-time.After(3 * time.Second):
		assert.Fail(t, "Callback on the server not called")
	}
	// Stop Client
	assert.NoError(t, clientConnection.Close())
	select {
	case <-closeCh:
	case <-time.After(3 * time.Second):
		assert.Fail(t, "Callback on the client not called")
	}
	assert.Error(t, clientConnection.Send(context.Background(), []byte(message)))
}
