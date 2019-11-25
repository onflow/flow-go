package protocols

import (
	"context"
	"fmt"
	"io"
	"net"
	"os"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"google.golang.org/grpc"
	"google.golang.org/grpc/encoding/gzip"

	"github.com/dapperlabs/flow-go/network/gossip"
	"github.com/dapperlabs/flow-go/proto/gossip/messages"
)

var (
	logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})
)

// Node interface defines the functions that any network node should have so that it can use Gserver
type Node interface {
	QueueService(ctx context.Context, msg *messages.GossipMessage) (*messages.GossipReply, error)
}

// clientStream represents a client with which Gserver establishes a stream connection
type clientStream interface {
	Send(*messages.GossipMessage) error
	Recv() (*messages.GossipReply, error)
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

// QueueService is invoked remotely using the gRPC stub,
// it receives a message from a remote node and places it inside the local nodes queue
// In current version of Gossip, StreamQueueService is utilized for direct 1-to-1 gossips
func (gs *Gserver) QueueService(ctx context.Context, msg *messages.GossipMessage) (*messages.GossipReply, error) {
	return gs.n.QueueService(context.Background(), msg)
}

// StreamQueueService receives sync data from stream and places is in queue
// In current version of Gossip, StreamQueueService is utilized for 1-to-many and 1-to-all gossips
func (gs *Gserver) StreamQueueService(saq messages.MessageReceiver_StreamQueueServiceServer) error {
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

		rep, err := gs.n.QueueService(context.Background(), req)

		if err != nil {
			return err
		}

		err = saq.Send(rep)
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
func (gs *Gserver) Place(ctx context.Context, addr string, msg *messages.GossipMessage, mode gossip.Mode) (*messages.GossipReply, error) {

	switch mode {
	case gossip.ModeOneToOne:
		return gs.sendDirect(ctx, addr, msg)
	case gossip.ModeOneToMany:
		return gs.streamToMany(ctx, addr, msg)
	case gossip.ModeOneToAll:
		return gs.streamToAll(ctx, addr, msg)
	default:
		return &messages.GossipReply{}, fmt.Errorf("unknown gossip mode: %v", mode)
	}

}

// sendDirect is used to send one-to-one direct messages
func (gs *Gserver) sendDirect(ctx context.Context, addr string, msg *messages.GossipMessage) (*messages.GossipReply, error) {
	conn, err := grpc.Dial(addr, grpc.WithInsecure())
	if err != nil {
		return &messages.GossipReply{}, fmt.Errorf("could not connect to %s: %v", addr, err)
	}
	defer closeConnection(conn)

	var reply *messages.GossipReply

	client := messages.NewMessageReceiverClient(conn)
	reply, err = client.QueueService(context.Background(), msg, grpc.UseCompressor(gzip.Name))
	if err != nil {
		return &messages.GossipReply{}, fmt.Errorf("error on gossiping the message to: %s, error: %v", addr, err)
	}
	return reply, nil

}

func (gs *Gserver) streamToAll(ctx context.Context, addr string, msg *messages.GossipMessage) (*messages.GossipReply, error) {
	stream, err := gs.cacheDial(addr, gossip.ModeOneToAll)
	if err != nil {
		return nil, fmt.Errorf("could not dial via cache: %v", err)
	}

	var reply *messages.GossipReply
	err = stream.Send(msg)
	if err == io.EOF {
		gs.cacheDialer.removeStream(addr)
		//TODO: Change to logging
		fmt.Println("Stream closed. Reopening...")
		return gs.streamToMany(context.Background(), addr, msg)
	}

	if err != nil {
		return &messages.GossipReply{}, fmt.Errorf("could not send message with stream %s: %v", addr, err)
	}
	reply, err = stream.Recv()

	if err != nil {
		return &messages.GossipReply{}, fmt.Errorf("error on gossiping the message to: %s, error: %v", addr, err)
	}
	return reply, nil

}

func (gs *Gserver) cacheDial(addr string, mode gossip.Mode) (clientStream, error) {
	return gs.cacheDialer.dial(addr, mode)
}

func (gs *Gserver) streamToMany(ctx context.Context, addr string, msg *messages.GossipMessage) (*messages.GossipReply, error) {
	stream, err := gs.cacheDial(addr, gossip.ModeOneToMany)
	if err != nil {
		return &messages.GossipReply{}, fmt.Errorf("could not dial server %s: %v", addr, err)
	}

	var reply *messages.GossipReply
	err = stream.Send(msg)
	if err == io.EOF {
		fmt.Println("Stream closed. Reopening..")
		// we assume a healthy stream would not ever being closed, hence, we mark a bad stream to be subject to a retry.
		// we also remove a bad stream from the cache to give it just a single more chance of try
		gs.cacheDialer.removeStream(addr)
		return gs.streamToMany(context.Background(), addr, msg)
	}

	if err != nil {
		return &messages.GossipReply{}, fmt.Errorf("could not send message with stream %s: %v", addr, err)
	}
	reply, err = stream.Recv()

	if err != nil {
		return &messages.GossipReply{}, fmt.Errorf("error on gossiping the message to: %s, error: %v", addr, err)
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
