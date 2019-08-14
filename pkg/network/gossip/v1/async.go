package async

import (
	"context"
	"fmt"
	"log"

	"net"

	"github.com/dapperlabs/bamboo-node/pkg/grpc/shared"
	proto "github.com/golang/protobuf/proto"
	"google.golang.org/grpc"
)

// QueueSize is the buffer size for holding incoming Gossip Messages
const QueueSize int = 10

// Node is holding the required information for a functioning async gossip node
type Node struct {
	address  string
	registry map[string]func(msg *shared.GossipMessage) error
	queue    chan *shared.GossipMessage
}

// NewNode returns a new gossip async node that can be used as a grpc service
func NewNode(address string) *Node {
	return &Node{address: address, registry: make(map[string]func(msg *shared.GossipMessage) error, 0), queue: make(chan *shared.GossipMessage, QueueSize)}
}

// RegisterFunc adds a new method to be used by GossipMessages
func (a *Node) RegisterFunc(name string, f func(msg *shared.GossipMessage) error) error {
	if _, ok := a.registry[name]; ok {
		return fmt.Errorf("function %v already registered", name)
	}

	a.registry[name] = f

	return nil
}

// Gossip sends a message to all peers specified in the given gossip message
// its only job is to place the message to the peers' queues
func (a *Node) Gossip(ctx context.Context, gossipMsg *shared.GossipMessage) ([]proto.Message, error) {
	var (
		gerrc    = make(chan error)
		repliesc = make(chan proto.Message)

		gerr    = &gossipError{}
		replies = make([]proto.Message, len(gossipMsg.Recipients))
	)

	// Loop through recipients
	for _, addr := range gossipMsg.Recipients {
		go func(addr string) {
			reply, err := a.place(ctx, addr, gossipMsg)
			gerrc <- err
			repliesc <- reply
		}(addr)
	}

	for i := range gossipMsg.Recipients {
		gerr.Collect(<-gerrc)
		replies[i] = <-repliesc
	}

	if len(*gerr) != 0 {
		return replies, gerr
	}

	return replies, nil
}

// Serve starts the async node's grpc server, and its sweeper as well
func (a *Node) Serve() {
	// Listen on the designated port and wait for rpc methods to be called
	lis, err := net.Listen("tcp", a.address)
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	// Send of the sweeper to monitor the queue
	go a.sweeper()

	s := grpc.NewServer()
	shared.RegisterMessageRecieverServer(s, a)
	if err := s.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}

// AsyncQueue places a given gossip message to the node's queue
// TODO: Maybe change the name to QueueMessage? or PlaceMessage?
func (a *Node) AsyncQueue(ctx context.Context, req *shared.GossipMessage) (*shared.VoidReply, error) {
	select {
	case <-ctx.Done():
		return &shared.VoidReply{}, fmt.Errorf("request timed out")
	case a.queue <- req:
		return &shared.VoidReply{}, nil
	}
}

// place encapsulates the process of placing a message to a peer's queue.
func (a *Node) place(ctx context.Context, addr string, gossipMsg *shared.GossipMessage) (proto.Message, error) {
	conn, err := grpc.Dial(addr, grpc.WithInsecure())
	if err != nil {
		return nil, fmt.Errorf("did not connect to %s: %v", addr, err)
	}

	defer conn.Close()

	client := shared.NewMessageRecieverClient(conn)
	reply, err := client.AsyncQueue(ctx, gossipMsg)
	if err != nil {
		return nil, fmt.Errorf("could not greet: %v", err)
	}

	return reply, nil
}

// sweeper looks for incoming message to the node's queue, and handles them
// concurrently
func (a *Node) sweeper() {
	for res := range a.queue {
		go a.messageHandler(res)
	}
}

// messageHandler is responsible to handle how to interpret incoming messages.
// TODO: discuss whether it could become something that the user of this library
// can pass. Similar to http handlers
func (a *Node) messageHandler(gossipMsg *shared.GossipMessage) error {
	if _, ok := a.registry[gossipMsg.Method]; !ok {
		return fmt.Errorf("function %v is not registered", gossipMsg.Method)
	}

	return a.registry[gossipMsg.Method](gossipMsg)
}

// gossipError is the error type used when sending multiple requests and awaiting
// multiple errors
type gossipError []error

func (g *gossipError) Collect(e error) {
	if e != nil {
		*g = append(*g, e)
	}
}

func (g *gossipError) Error() string {
	err := "gossip errors:\n"
	for i, e := range *g {
		err += fmt.Sprintf("\terror %d: %s\n", i, e.Error())
	}

	return err
}
