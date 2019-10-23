package protocols

import (
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"sync"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"google.golang.org/grpc"

	"github.com/dapperlabs/flow-go/network/gossip"
	"github.com/dapperlabs/flow-go/proto/gossip/messages"
)

var (
	logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})
)

// Node interface defines the functions that any network node should have so that it can use Gserver
type Node interface {
	SyncQueue(ctx context.Context, msg *messages.GossipMessage) (*messages.GossipReply, error)
	AsyncQueue(ctx context.Context, msg *messages.GossipMessage) (*messages.GossipReply, error)
}

// clientStream represents a client with which Gserver establishes a stream connection
type clientStream interface {
	Send(*messages.GossipMessage) error
	Recv() (*messages.GossipReply, error)
	grpc.ClientStream
}

// Gserver represents a gRPC server and a client
type Gserver struct {
	n       Node
	streams map[string]clientStream
	mu      sync.Mutex
}

// NewGServer returns a new Gserver instance
func NewGServer(n Node) *Gserver {
	return &Gserver{
		streams: make(map[string]clientStream),
		n:       n,
	}
}

// SyncQueue is invoked remotely using the gRPC stub,
// it receives a message from a remote node and places it inside the local nodes queue
func (gs *Gserver) SyncQueue(ctx context.Context, msg *messages.GossipMessage) (*messages.GossipReply, error) {
	return gs.n.SyncQueue(ctx, msg)
}

// AsyncQueue is invoked remotely using the gRPC stub,
// it receives a message from a remote node and places it inside the local nodes queue
func (gs *Gserver) AsyncQueue(ctx context.Context, msg *messages.GossipMessage) (*messages.GossipReply, error) {
	return gs.n.AsyncQueue(ctx, msg)
}

// StreamAsyncQueue receives sync data from stream and places is in queue
func (gs *Gserver) StreamAsyncQueue(saq messages.MessageReceiver_StreamAsyncQueueServer) error {
	ctx := saq.Context()
	for {

		// exit if context is done
		// or continue
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// receive data from stream
		req, err := saq.Recv()
		if err == io.EOF {
			// return will close stream from server side
			logger.Info().Msg("exiting StreamAsyncQueue")
			return nil
		}
		if err != nil {
			logger.Debug().Err(fmt.Errorf("receive error: %v", err)).Send()
			continue
		}

		rep, err := gs.n.AsyncQueue(ctx, req)
		if err != nil {
			return err
		}
		err = saq.Send(rep)
		if err != nil {
			return err
		}
	}
}

// StreamSyncQueue recieves async data from stream and places it in queue
func (gs *Gserver) StreamSyncQueue(ssq messages.MessageReceiver_StreamSyncQueueServer) error {
	ctx := ssq.Context()
	for {

		// exit if context is done
		// or continue
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// receive data from stream
		req, err := ssq.Recv()
		if err == io.EOF {
			// return will close stream from server side
			logger.Info().Msg("exiting StreamAsyncQueue")
			return nil
		}
		if err != nil {
			logger.Debug().Err(fmt.Errorf("receive error: %v", err)).Send()
			continue
		}

		rep, err := gs.n.SyncQueue(ctx, req)
		if err != nil {
			return err
		}
		err = ssq.Send(rep)
		if err != nil {
			return err
		}
	}
}

// Serve starts serving a new connection
func (gs *Gserver) Serve(listener net.Listener) {
	s := grpc.NewServer()
	messages.RegisterMessageReceiverServer(s, gs)
	if err := s.Serve(listener); err != nil {
		logger.Debug().Err(fmt.Errorf("failed to serve: %v", err)).Send()
	}
}

// Place places a message for sending according to the gossip mode
func (gs *Gserver) Place(ctx context.Context, addr string, msg *messages.GossipMessage, isSynchronous bool, mode gossip.Mode) (*messages.GossipReply, error) {

	switch mode {
	case gossip.ModeOneToOne:
		return gs.place(ctx, addr, msg, isSynchronous, mode)
	case gossip.ModeOneToAll:
		return gs.placeStream(ctx, addr, msg, isSynchronous, mode)
	default:
		return &messages.GossipReply{}, fmt.Errorf("Unimplemented mode")
	}

}

// place is used to send one-to-one direct messages
func (gs *Gserver) place(ctx context.Context, addr string, msg *messages.GossipMessage, isSynchronous bool, mode gossip.Mode) (*messages.GossipReply, error) {
	conn, err := grpc.Dial(addr, grpc.WithInsecure())
	if err != nil {
		return &messages.GossipReply{}, fmt.Errorf("could not connect to %s: %v", addr, err)
	}
	defer closeConnection(conn)

	var reply *messages.GossipReply

	client := messages.NewMessageReceiverClient(conn)
	if isSynchronous {
		reply, err = client.SyncQueue(ctx, msg)
	} else {
		reply, err = client.AsyncQueue(ctx, msg)
	}
	if err != nil {
		return &messages.GossipReply{}, fmt.Errorf("error on gossiping the message to: %s, error: %v", addr, err)
	}
	return reply, nil

}

// placeStream is used to send messages using streams
//todo we need to elaborate on this in comming issues
func (gs *Gserver) placeStream(ctx context.Context, addr string, msg *messages.GossipMessage, isSynchronous bool, mode gossip.Mode) (*messages.GossipReply, error) {
	var stream clientStream
	gs.mu.Lock()
	defer gs.mu.Unlock()

	// if there alread exists a stream then use it
	if val, ok := gs.streams[addr]; ok {
		stream = val
	} else {
		// otherwise create a new stream
		conn, err := grpc.Dial(addr, grpc.WithInsecure())
		if err != nil {
			return &messages.GossipReply{}, fmt.Errorf("could not connect to %s: %v", addr, err)
		}
		client := messages.NewMessageReceiverClient(conn)
		if isSynchronous {
			stream, err = client.StreamSyncQueue(ctx)
		} else {
			stream, err = client.StreamAsyncQueue(ctx)
		}
		if err != nil {
			return &messages.GossipReply{}, fmt.Errorf("could not start grpc stream with server %s: %v", addr, err)
		}
		gs.streams[addr] = stream
	}

	var reply *messages.GossipReply
	err := stream.Send(msg)
	if err != nil {
		return &messages.GossipReply{}, fmt.Errorf("could not send message with stream %s: %v", addr, err)
	}
	reply, err = stream.Recv()

	if err != nil {
		return &messages.GossipReply{}, fmt.Errorf("error on gossiping the message to: %s, error: %v", addr, err)
	}
	return reply, nil

}

//closeConnection closes a grpc client connection and prints the errors, if any,
func closeConnection(conn *grpc.ClientConn) {
	err := conn.Close()
	if err != nil {
		logger.Debug().Err(fmt.Errorf("Error closing grpc connection %v", err)).Send()
	}
}
