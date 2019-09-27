package gnode

import (
	"context"
	"fmt"
	"log"
	"net"

	"google.golang.org/grpc"

	"github.com/dapperlabs/flow-go/pkg/grpc/shared"
	"github.com/dapperlabs/flow-go/pkg/network/gossip"
)

// QueueSize is the buffer size for holding incoming Gossip Messages
const QueueSize int = 10

// To make sure that Node complies with the gossip.Service interface
var _ gossip.Service = (*Node)(nil)

// Node is holding the required information for a functioning async gossip node
type Node struct {
	regMngr *registryManager
	tracker messageTracker

	queue chan *shared.GossipMessage
}

// NewNode returns a new gossip node that can be used as a grpc service, with an
// empty registry of methods
func NewNode() *Node {
	return &Node{
		regMngr: newEmptyRegistryManager(),
		queue:   make(chan *shared.GossipMessage, QueueSize),

		tracker: make(messageTracker, 0),
	}
}

// NewNodeWithRegistry returns a new gossip node that can be used as a grpc service, given a
// specific registry of methods to provide for callers
func NewNodeWithRegistry(registry Registry) *Node {
	return &Node{
		regMngr: newRegistryManager(registry),
		queue:   make(chan *shared.GossipMessage, QueueSize),

		tracker: make(messageTracker, 0),
	}
}

// SyncGossip sends a message to all peers and waits for their reply
func (n *Node) SyncGossip(ctx context.Context, payload []byte, recipients []string, method string) ([]*shared.GossipReply, error) {
	return n.gossip(ctx, payload, recipients, method, true)
}

// AsyncGossip sends a message to all peers without awaiting for their response
func (n *Node) AsyncGossip(ctx context.Context, payload []byte, recipients []string, method string) ([]*shared.GossipReply, error) {
	return n.gossip(ctx, payload, recipients, method, false)
}

// AsyncQueue places a given gossip message to the node's queue
func (n *Node) AsyncQueue(ctx context.Context, req *shared.GossipMessage) (*shared.GossipReply, error) {
	select {
	case <-ctx.Done():
		return &shared.GossipReply{}, fmt.Errorf("request timed out")
	case n.queue <- req:
		return &shared.GossipReply{}, nil
	}
}

// SyncQueue places a given gossip message to the node's queue and blocks waiting for a reply.
func (n *Node) SyncQueue(ctx context.Context, req *shared.GossipMessage) (*shared.GossipReply, error) {
	select {
	case <-ctx.Done():
		return &shared.GossipReply{}, fmt.Errorf("request timed out")
	case n.queue <- req:
	}

	uuid := req.Uuid

	if err := n.tracker.TrackMessage(uuid); err != nil {
		return &shared.GossipReply{}, fmt.Errorf("could not track message %v: %v", uuid, err)
	}

	select {
	case <-ctx.Done():
		return &shared.GossipReply{}, fmt.Errorf("request timed out")
	case <-n.tracker.Done(uuid):
		msgReply, msgErr, err := n.tracker.RetrieveReply(uuid)
		if err != nil {
			return &shared.GossipReply{}, err
		}

		return &shared.GossipReply{ResponseByte: msgReply}, msgErr
	}
}

// Serve starts the async node's grpc server, and its sweeper as well
func (n *Node) Serve(listner net.Listener) {
	// Send of the sweeper to monitor the queue
	go n.sweeper()

	s := grpc.NewServer()
	shared.RegisterMessageRecieverServer(s, n)
	if err := s.Serve(listner); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}

// RegisterFunc allows the addition of new methods to the node's registry
func (n *Node) RegisterFunc(method string, f HandleFunc) error {
	return n.regMngr.AddMethod(method, f)
}

func (n *Node) gossip(ctx context.Context, payload []byte, recipients []string, method string, sync bool) ([]*shared.GossipReply, error) {

	gossipMsg, err := generateGossipMessage(payload, recipients, method)
	if err != nil {
		return nil, fmt.Errorf("could not generate gossip message: %v", err)
	}

	var (
		gerrc    = make(chan error)
		repliesc = make(chan *shared.GossipReply)

		gerr    = &gossipError{}
		replies = make([]*shared.GossipReply, len(gossipMsg.Recipients))
	)

	// Loop through recipients
	for _, addr := range gossipMsg.Recipients {
		go func(addr string) {
			reply, err := n.placeMessage(ctx, addr, gossipMsg, sync)
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

// placeMessage encapsulates the process of placing a message to a peer's queue.
func (n *Node) placeMessage(ctx context.Context, addr string, gossipMsg *shared.GossipMessage, sync bool) (*shared.GossipReply, error) {
	conn, err := grpc.Dial(addr, grpc.WithInsecure())
	if err != nil {
		return nil, fmt.Errorf("did not connect to %s: %v", addr, err)
	}

	defer conn.Close()

	client := shared.NewMessageRecieverClient(conn)

	var reply *shared.GossipReply

	if sync {
		reply, err = client.SyncQueue(ctx, gossipMsg)
	} else {
		reply, err = client.AsyncQueue(ctx, gossipMsg)
	}

	if err != nil {
		return nil, fmt.Errorf("could not greet: %v", err)
	}

	return reply, nil
}

// sweeper looks for incoming message to the node's queue, and handles them
// concurrently
func (n *Node) sweeper() {
	for res := range n.queue {
		go n.messageHandler(res)
	}
}

// messageHandler is responsible to handle how to interpret incoming messages.
func (n *Node) messageHandler(gossipMsg *shared.GossipMessage) error {
	uuid := gossipMsg.Uuid

	invokeResp, err := n.regMngr.Invoke(gossipMsg.Method, gossipMsg.Payload)
	if err != nil {
		return fmt.Errorf("could not invoke: %v", err)
	}

	return n.tracker.FillMessageReply(uuid, invokeResp.Resp, invokeResp.Err)
}
