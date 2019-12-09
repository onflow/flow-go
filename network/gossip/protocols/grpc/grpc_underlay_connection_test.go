package protocols

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/dapperlabs/flow-go/network/gossip"
)

func TestGRPCUnderlayConnection_Send(t *testing.T) {
	var underlay gossip.Underlay = &GRPCUnderlay{}
	address := "127.0.0.1:50000"
	// Start the Server
	go func() {
		assert.NoError(t, underlay.Start(address))
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
	clientConnection, err := underlay.Dial(address)
	assert.NotNil(t, clientConnection)
	assert.NoError(t, err)
	message := "hello from client"
	assert.NoError(t, clientConnection.Send(context.Background(), []byte(message)))
	recvdMessage := <-ch
	assert.Equal(t, message, string(recvdMessage))
	// Stop Client
	assert.NoError(t, clientConnection.Close())
}

func TestGRPCUnderlayConnection_OnClosed(t *testing.T) {
	var underlay gossip.Underlay = &GRPCUnderlay{}
	// Setup the server call back function
	ch := make(chan []byte)
	callbackfunc := func(sender string, msg []byte) {
		ch <- msg
	}
	assert.NoError(t, underlay.Handle(callbackfunc))
	address := "127.0.0.1:50000"
	// Start the Server
	go func() {
		assert.NoError(t, underlay.Start(address))
	}()
	// Stop Server at the end
	defer underlay.Stop()
	// Start the Client
	clientConnection, err := underlay.Dial(address)
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
