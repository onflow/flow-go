package protocols

import (
	"context"
	"fmt"
	"io"
	"net"
	"os"

	// Initialize the gzip compressor
	"github.com/dapperlabs/flow-go/pkg/grpc/shared"
	"github.com/dapperlabs/flow-go/pkg/network/gossip"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"google.golang.org/grpc"
	"google.golang.org/grpc/encoding/gzip"
)

var (
	logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})
)

// Node interface defines the functions that any network node should have so that it can use Gserver
type Node interface {
	SyncQueue(ctx context.Context, msg *shared.GossipMessage) (*shared.GossipReply, error)
	AsyncQueue(ctx context.Context, msg *shared.GossipMessage) (*shared.GossipReply, error)
}

// clientStream represents a client with which Gserver establishes a stream connection
type clientStream interface {
	Send(*shared.GossipMessage) error
	Recv() (*shared.GossipReply, error)
	grpc.ClientStream
}

// Gserver represents a gRPC server and a client
type Gserver struct {
	n           Node
	streams     map[string]clientStream
	cacheDialer *CacheDialer
}

// NewGServer returns a new Gserver instance
func NewGServer(n Node) (*Gserver, error) {

	cd, err := NewCacheDialer(100)
	if err != nil {
		return nil, fmt.Errorf("could not create a cachedialer: %v", err)
	}

	return &Gserver{
		streams:     make(map[string]clientStream),
		n:           n,
		cacheDialer: cd,
	}, nil
}

// SyncQueue is invoked remotely using the gRPC stub,
// it receives a message from a remote node and places it inside the local nodes queue
func (gs *Gserver) SyncQueue(ctx context.Context, msg *shared.GossipMessage) (*shared.GossipReply, error) {
	return gs.n.SyncQueue(ctx, msg)
}

// AsyncQueue is invoked remotely using the gRPC stub,
// it receives a message from a remote node and places it inside the local nodes queue
func (gs *Gserver) AsyncQueue(ctx context.Context, msg *shared.GossipMessage) (*shared.GossipReply, error) {
	return gs.n.AsyncQueue(ctx, msg)
}

// StreamAsyncQueue receives sync data from stream and places is in queue
func (gs *Gserver) StreamAsyncQueue(saq shared.MessageReceiver_StreamAsyncQueueServer) error {
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

		if err := saq.Send(rep); err != nil {
			return err
		}
	}
}

// StreamSyncQueue recieves async data from stream and places it in queue
func (gs *Gserver) StreamSyncQueue(ssq shared.MessageReceiver_StreamSyncQueueServer) error {
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

		if err := ssq.Send(rep); err != nil {
			return err
		}
	}
}

// Serve starts serving a new connection
func (gs *Gserver) Serve(listener net.Listener) {
	s := grpc.NewServer()
	shared.RegisterMessageReceiverServer(s, gs)
	if err := s.Serve(listener); err != nil {
		logger.Debug().Err(fmt.Errorf("failed to serve: %v", err)).Send()
	}
}

// Place places a message for sending according to the gossip mode
func (gs *Gserver) Place(ctx context.Context, addr string, msg *shared.GossipMessage, isSynchronous bool, mode gossip.Mode) (*shared.GossipReply, error) {

	switch mode {
	case gossip.ModeOneToOne:
		return gs.place(ctx, addr, msg, isSynchronous)
	case gossip.ModeOneToMany:
		return gs.placeStreamToMany(ctx, addr, msg, isSynchronous)
	case gossip.ModeOneToAll:
		return gs.placeStreamToAll(ctx, addr, msg, isSynchronous)
	default:
		return &shared.GossipReply{}, fmt.Errorf("Unimplemented mode")
	}

}

// place is used to send one-to-one direct messages
func (gs *Gserver) place(ctx context.Context, addr string, msg *shared.GossipMessage, isSynchronous bool) (*shared.GossipReply, error) {
	conn, err := grpc.Dial(addr, grpc.WithInsecure())
	if err != nil {
		return &shared.GossipReply{}, fmt.Errorf("could not connect to %s: %v", addr, err)
	}
	defer closeConnection(conn)

	var reply *shared.GossipReply

	client := shared.NewMessageReceiverClient(conn)
	if isSynchronous {
		reply, err = client.SyncQueue(ctx, msg, grpc.UseCompressor(gzip.Name))
	} else {
		reply, err = client.AsyncQueue(ctx, msg, grpc.UseCompressor(gzip.Name))
	}
	if err != nil {
		return &shared.GossipReply{}, fmt.Errorf("error on gossiping the message to: %s, error: %v", addr, err)
	}
	return reply, nil

}


func (gs *Gserver) placeStreamToAll(ctx context.Context, addr string, msg *shared.GossipMessage, isSynchronous bool) (*shared.GossipReply, error) {

	stream, err := gs.cacheDial(addr, isSynchronous, gossip.ModeOneToAll)
	if err != nil {
		return nil, fmt.Errorf("could not dial via cache: %v", err)
	}

	var reply *shared.GossipReply
	err = stream.Send(msg)
	if err == io.EOF {
		gs.cacheDialer.removeStream(addr)
		fmt.Println("Stream closed. Reoppenining...")
		return gs.placeStreamToMany(ctx, addr, msg, isSynchronous)
	}

	if err != nil {
		return &shared.GossipReply{}, fmt.Errorf("could not send message with stream %s: %v", addr, err)
	}
	reply, err = stream.Recv()

	if err != nil {
		return &shared.GossipReply{}, fmt.Errorf("error on gossiping the message to: %s, error: %v", addr, err)
	}
	return reply, nil

}

func (gs *Gserver) cacheDial(addr string, isSynchronous bool, mode gossip.Mode) (clientStream, error) {
	return gs.cacheDialer.dial(addr, isSynchronous, mode)
}

func (gs *Gserver) placeStreamToMany(ctx context.Context, addr string, msg *shared.GossipMessage, isSynchronous bool) (*shared.GossipReply, error) {

	stream, err := gs.cacheDial(addr, isSynchronous, gossip.ModeOneToMany)
	if err != nil {
		return &shared.GossipReply{}, fmt.Errorf("could not dial server %s: %v", addr, err)
	}

	var reply *shared.GossipReply
	err = stream.Send(msg)
	if err == io.EOF {
		fmt.Println("Stream closed. Reoppenining...")
		// we assume a healthy stream would not ever being closed, hence, we mark a bad stream to be subject to a retry.
		// we also remove a bad stream from the cache to give it just a single more chance of try
		gs.cacheDialer.removeStream(addr)
		return gs.placeStreamToMany(ctx, addr, msg, isSynchronous)
	}

	if err != nil {
		return &shared.GossipReply{}, fmt.Errorf("could not send message with stream %s: %v", addr, err)
	}
	reply, err = stream.Recv()

	if err != nil {
		return &shared.GossipReply{}, fmt.Errorf("error on gossiping the message to: %s, error: %v", addr, err)
	}
	return reply, nil

}

// closeConnection closes a grpc client connection and prints the errors, if any,
func closeConnection(conn *grpc.ClientConn) {
	err := conn.Close()
	if err != nil {
		logger.Debug().Err(fmt.Errorf("Error closing grpc connection %v", err)).Send()
	}
}
