// Package protocols is a GRPC implementation of the Underlay interface.
package protocols

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"

	"google.golang.org/grpc"
	"google.golang.org/grpc/encoding/gzip"
	"google.golang.org/grpc/peer"

	"github.com/dapperlabs/flow-go/network/gossip"
	"github.com/dapperlabs/flow-go/proto/gossip/messages"
)

// Compile time verification that GRPCUnderlay implements the two interfaces
var _ gossip.Underlay = &GRPCUnderlay{}
var _ messages.MessageReceiverServer = &GRPCUnderlay{}

type GRPCUnderlay struct {
	grpcServer *grpc.Server
	onReceive  gossip.OnReceiveCallback
}

func (u *GRPCUnderlay) Handle(onReceive gossip.OnReceiveCallback) error {
	if u.onReceive != nil {
		return fmt.Errorf(" Handle call back is already set")
	}
	u.onReceive = onReceive
	return nil
}

func (u *GRPCUnderlay) Dial(address string) (gossip.Connection, error) {
	conn, err := grpc.Dial(address, grpc.WithInsecure())
	if err != nil {
		return nil, fmt.Errorf("could not connect to %s: %v", address, err)
	}
	client := messages.NewMessageReceiverClient(conn)
	stream, err := client.StreamQueueService(context.Background(), grpc.UseCompressor(gzip.Name))
	gossipConnection := &GRPCUnderlayConnection{grpcClientStream: stream}
	return gossipConnection, err
}

func (u *GRPCUnderlay) Start(address string) error {
	listener, err := net.Listen("tcp4", address)
	if err != nil {
		return fmt.Errorf("invalid address %s: %v", address, err)
	}
	return u.StartWithListener(&listener)
}

func (u *GRPCUnderlay) StartWithListener(listener *net.Listener) error {
	if u.grpcServer != nil {
		return fmt.Errorf("GRPC server already started")
	}
	if u.onReceive == nil {
		return fmt.Errorf("OnReceive call has not been set")
	}
	grpcServer := grpc.NewServer()
	messages.RegisterMessageReceiverServer(grpcServer, u)

	u.grpcServer = grpcServer

	// Blocking call
	if err := grpcServer.Serve(*listener); err != nil {
		return fmt.Errorf("failed to serve: %v", err)
	}

	return nil
}

func (u *GRPCUnderlay) Stop() error {
	if u.grpcServer == nil {
		return errors.New(" GRPC Server set to nil ")
	}

	u.grpcServer.GracefulStop()
	return nil
}

// QueueService is invoked remotely using the gRPC stub,
// it receives a message from a remote node and places it inside the local nodes queue
// In current version of Gossip, StreamQueueService is utilized for direct 1-to-1 gossips
func (u *GRPCUnderlay) QueueService(ctx context.Context, msg *messages.GossipMessage) (*messages.GossipReply, error) {
	return nil, nil
}

// StreamQueueService receives sync data from stream and places is in queue
// In current version of Gossip, StreamQueueService is utilized for 1-to-many and 1-to-all gossips
func (u *GRPCUnderlay) StreamQueueService(saq messages.MessageReceiver_StreamQueueServiceServer) error {
	ctx := saq.Context()
	for {
		// exit if context is done
		// or continue
		select {
		case <-ctx.Done():
			return nil
		default:
		}

		// receive data from stream
		req, err := saq.Recv()

		if err == io.EOF {
			// return will close stream from server side
			return nil
		}
		if err != nil {
			return err
		}
		peer, _ := peer.FromContext(ctx)
		if u.onReceive == nil {
			return fmt.Errorf("handle call back has not been set")
		}
		u.onReceive(peer.Addr.String(), req.Payload)
		return nil
	}
}
