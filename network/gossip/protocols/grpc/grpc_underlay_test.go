package protocols

import (
	"context"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/dapperlabs/flow-go/network/gossip"
	"github.com/dapperlabs/flow-go/proto/gossip/messages"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
)

func TestGRPCUnderlay_Start_Twice(t *testing.T) {
	var protocol gossip.Underlay = &GRPCUnderlay{}
	require.NotNil(t, protocol, "Protocol is nil")
	protocol.Handle(func(sender string, msg []byte) {})
	address := ":0"
	listener, _ := net.Listen("tcp4", address)
	go func() {
		require.NoError(t, protocol.StartWithListener(listener))
	}()
	time.Sleep(time.Duration(1))
	defer protocol.Stop()
	checkClientConnection(t, listener.Addr().String())
	assert.Error(t, protocol.StartWithListener(listener))
}

func TestGRPCUnderlay_Start_Stop(t *testing.T) {
	var protocol gossip.Underlay = &GRPCUnderlay{}
	require.NotNil(t, protocol, "Protocol is nil")
	protocol.Handle(func(sender string, msg []byte) {})
	address := ":0"
	listener, _ := net.Listen("tcp4", address)
	go func() {
		require.NoError(t, protocol.StartWithListener(listener))
	}()
	checkClientConnection(t, listener.Addr().String())
	assert.NoError(t, protocol.Stop())
}

func TestGRPCUnderlay_Handle(t *testing.T) {
	var protocol gossip.Underlay = &GRPCUnderlay{}
	type Tuple struct {
		sender string
		msg    []byte
	}
	ch := make(chan Tuple)
	callbackfunc := func(sender string, msg []byte) {
		ch <- Tuple{sender: sender, msg: msg}
	}
	assert.NoError(t, protocol.Handle(callbackfunc))
	address := ":0"
	listener, _ := net.Listen("tcp4", address)
	go func() {
		require.NoError(t, protocol.StartWithListener(listener))
	}()
	checkClientConnection(t, listener.Addr().String())
	defer protocol.Stop()
	conn, err := createClientConnection(listener.Addr().String())
	assert.NoError(t, err)
	client := messages.NewMessageReceiverClient(conn)
	stream, err := client.StreamQueueService(context.Background())
	messagePayload := "hello"
	gossipMessage := &messages.GossipMessage{Payload: []byte(messagePayload)}
	err = stream.Send(gossipMessage)
	assert.NoError(t, err)
	select {
	case recvdMessage := <-ch:
		assert.Equal(t, messagePayload, string(recvdMessage.msg))
		assert.True(t, strings.HasPrefix(recvdMessage.sender, "127.0.0.1"))
	case <-time.After(3 * time.Second):
		assert.Fail(t, "Callback not called")
	}
}

func createClientConnection(address string) (*grpc.ClientConn, error) {
	return grpc.Dial(address, grpc.WithInsecure())
}

func checkClientConnection(t *testing.T, address string) {
	timeout := 5 * time.Millisecond
	assert.Eventually(t, func() bool {
		con, err := net.DialTimeout("tcp", address, timeout)
		defer con.Close()
		return con != nil && err == nil
	}, 4*timeout, 2*timeout)
}
