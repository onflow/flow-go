package protocols

import (
	"context"
	"fmt"
	"github.com/dapperlabs/flow-go/network/gossip"
	"github.com/dapperlabs/flow-go/proto/gossip/messages"
)

var _ gossip.Connection = &GRPCUnderlayConnection{}

// GRPCUnderlayConnection implements UnderlayConnection
type GRPCUnderlayConnection struct {
	grpcClientStream messages.MessageReceiver_StreamQueueServiceClient
	onCloseFunc      func()
}

// Send sends a message using the GRPC Stream
func (grpcUnderlayConnection *GRPCUnderlayConnection) Send(ctx context.Context, msg []byte) error {
	return grpcUnderlayConnection.grpcClientStream.Send(&messages.GossipMessage{Payload: msg})
}

// OnClosed calls the callback to be exectuded after the connection is closed
func (grpcUnderlayConnection *GRPCUnderlayConnection) OnClosed(onCloseFunc func()) error {
	if grpcUnderlayConnection.onCloseFunc != nil {
		return fmt.Errorf(" OnClose call back is already set")
	}
	grpcUnderlayConnection.onCloseFunc = onCloseFunc
	return nil
}

// Close shuts down the connection
func (grpcUnderlayConnection *GRPCUnderlayConnection) Close() error {
	// Kick off the onClose function if we have one in a separate go routine
	if grpcUnderlayConnection.onCloseFunc != nil {
		onCloseFuncRoutine := func() {
			go grpcUnderlayConnection.onCloseFunc()
		}
		defer onCloseFuncRoutine()
	}
	return grpcUnderlayConnection.grpcClientStream.CloseSend()
}
