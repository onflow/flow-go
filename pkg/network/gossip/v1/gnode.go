package gnode

import (
	"context"
	"fmt"
	"github.com/dapperlabs/flow-go/pkg/grpc/shared"
	"github.com/dapperlabs/flow-go/pkg/network/gossip"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"google.golang.org/grpc"
	"net"
	"os"
	"reflect"
)

// QueueSize is the buffer size of the node for holding incoming Gossip Messages
// Once buffer of a node gets full, it does not accept incoming messages
const QueueSize int = 10

// To make sure that Node complies with the gossip.Service interface
var _ gossip.Service = (*Node)(nil)

// Node is holding the required information for a functioning async gossip node
type Node struct {
	logger  zerolog.Logger
	regMngr *registryManager
	tracker messageTracker
	queue   chan *entry
}

// entry stores information parameters necessary to process a received message
type entry struct {
	msg *shared.GossipMessage
	ctx context.Context
}


// NewNode returns a new instance of a Gossip node with a predefined logger and a predefined
// registry of message types passing nil instead of the messageTypeRegistry results
// in creation of an empty registry for the node.
func NewNode(logger zerolog.Logger, msgTypesRegistry Registry) *Node {
	node := &Node{
		logger:  logger,
		regMngr: newRegistryManager(msgTypesRegistry),
		queue:   make(chan *entry, QueueSize),
		tracker: make(messageTracker, 0),
	}


	if reflect.DeepEqual(node.logger, zerolog.Logger{}){
		// enabling human-friendly and colorized output logging in case the default logger is
		// in its zero value state
		node.logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr}).With().Caller().Logger()
	}

	return node
}

// SyncGossip synchronizes over the reply of recipients
// i.e., it sends a message to all recipients and blocks for their reply
func (n *Node) SyncGossip(ctx context.Context, payload []byte, recipients []string, msgType string) ([]*shared.GossipReply, error) {
	return n.gossip(ctx, payload, recipients, msgType, true)
}

// AsyncGossip synchronizes over the delivery
// i.e., sends a message to all recipients, and only blocks for delivery without blocking for their response
func (n *Node) AsyncGossip(ctx context.Context, payload []byte, recipients []string, msgType string) ([]*shared.GossipReply, error) {
	return n.gossip(ctx, payload, recipients, msgType, false)
}

// AsyncQueue is invoked remotely using the gRPC stub,
// it receives a message from a remote node and places it inside the local nodes queue
// it is synchronized with the remote node on the message reception (and NOT reply), i.e., blocks the remote node until either
// a timeout or placement of the message into the queue
func (n *Node) AsyncQueue(ctx context.Context, msg *shared.GossipMessage) (*shared.GossipReply, error) {

	log := n.logger.With().
		Str("function", "AsyncQueue").
		Logger()

	//if context timed out already, return an error
	select {
	case <-ctx.Done():
		log.Debug().Err(ErrTimedOut).Send()
		return &shared.GossipReply{}, ErrTimedOut
	default:
	}

	// Wait until the context expires, or if the msg got placed on the queue
	select {
	case <-ctx.Done():
		log.Debug().Err(ErrTimedOut).Send()
		return &shared.GossipReply{}, ErrTimedOut
	case n.queue <- &entry{ctx: ctx, msg: msg}:
		return &shared.GossipReply{}, nil
	}
}

// SyncQueue is invoked remotely using the gRPC stub,
// it receives a message from a remote node and places it inside the local nodes queue
// it is synchronized with the remote node on the message reply, i.e., blocks the remote node until either
// a timeout or a reply is getting prepared
func (n *Node) SyncQueue(ctx context.Context, msg *shared.GossipMessage) (*shared.GossipReply, error) {

	log := n.logger.With().
		Str("function", "SyncQueue").
		Logger()

	//if context timed out already, return an error
	select {
	case <-ctx.Done():
		log.Debug().Err(ErrTimedOut).Send()
		return &shared.GossipReply{}, ErrTimedOut
	default:
	}

	//getting blocked until either a timeout,
	// or the message getting placed into the local nodes queue
	select {
	case <-ctx.Done():
		log.Debug().Err(ErrTimedOut).Send()
		return &shared.GossipReply{}, ErrTimedOut
	case n.queue <- &entry{ctx: ctx, msg: msg}:
	}

	messageUUID := msg.Uuid

	if err := n.tracker.TrackMessage(messageUUID); err != nil {
		err := fmt.Errorf("could not track message %v: %v", messageUUID, err)
		log.Error().Err(err).Send()
		return &shared.GossipReply{}, err
	}

	// getting blocked until either a timeout
	// or the message getting processed out of the queue of the local node
	select {
	case <-ctx.Done():
		log.Debug().Err(ErrTimedOut).Send()
		return &shared.GossipReply{}, ErrTimedOut
	case <-n.tracker.Done(messageUUID):
		msgReply, msgErr, err := n.tracker.GetReply(messageUUID)
		if err != nil {
			return &shared.GossipReply{}, err
		}
		return &shared.GossipReply{ResponseByte: msgReply}, msgErr
	}
}

// Serve starts an async node grpc server, and its sweeper as well
func (n *Node) Serve(listener net.Listener) error {
	log := n.logger.With().
		Str("function", "Serve").
		Logger()

	// Send of the sweeper to monitor the queue
	go n.sweeper()

	s := grpc.NewServer()
	shared.RegisterMessageReceiverServer(s, n)
	if err := s.Serve(listener); err != nil {
		//todo for v1.2: investigate making this a panic
		err := fmt.Errorf("failed to serve: %v", err)
		log.Panic().Err(err).Send()
		return err
	}

	return nil
}

// RegisterFunc allows the addition of new message types to the node's registry
func (n *Node) RegisterFunc(msgType string, f HandleFunc) error {
	return n.regMngr.AddMessageType(msgType, f)
}

func (n *Node) gossip(ctx context.Context, payload []byte, recipients []string, msgType string, isSynchronous bool) ([]*shared.GossipReply, error) {
	log := n.logger.With().
		Str("function", "gossip").
		Logger()

	gossipMsg, err := generateGossipMessage(payload, recipients, msgType)
	if err != nil {
		err := fmt.Errorf("could not generate gossip message:  %v", err)
		log.Error().Err(err).Send()
		return nil, err
	}

	gossipErrChan := make(chan error)
	gossipRepliesChan := make(chan *shared.GossipReply)
	gossipErr := &gossipError{}
	gossipReplies := make([]*shared.GossipReply, len(gossipMsg.Recipients))

	// Todo for v1.2
	// Note: The order of gossip replies and errors is not defined. Perhaps
	// this is something we need to define for later, as currently there is no clear way
	// to match errors to responses.
	// Loop through recipients
	for _, addr := range gossipMsg.Recipients {
		go func(addr string) {
			reply, err := n.placeMessage(ctx, addr, gossipMsg, isSynchronous)
			gossipErrChan <- err
			gossipRepliesChan <- reply
		}(addr)
	}

	// Append errors and replies from the respective channels
	for i := range gossipMsg.Recipients {
		gossipErr.Append(<-gossipErrChan)
		gossipReplies[i] = <-gossipRepliesChan
	}

	// If encountered errors, then return them to the caller
	if len(*gossipErr) != 0 {
		return gossipReplies, gossipErr
	}

	return gossipReplies, nil
}

// placeMessage is invoked by the node, and encapsulates the process of placing a single message to a recipient's incoming queue
// placeMessage takes a context, address of target node, the message to send, and
// whether to use sync or async mode (true for sync, false for async)
// placeMessage establishes and manages a gRPC client inside
func (n *Node) placeMessage(ctx context.Context, addr string, msg *shared.GossipMessage, isSynchronous bool) (*shared.GossipReply, error) {
	log := n.logger.With().
		Str("function", "placeMessage").
		Str("address", addr).
		Logger()

	conn, err := grpc.Dial(addr, grpc.WithInsecure())
	if err != nil {
		err := fmt.Errorf("could not connect to %s: %v", addr, err)
		log.Error().Err(err).Send()
		return nil, err
	}
	defer closeConnection(conn)
	client := shared.NewMessageReceiverClient(conn)
	var reply *shared.GossipReply
	if isSynchronous {
		reply, err = client.SyncQueue(ctx, msg)
	} else {
		reply, err = client.AsyncQueue(ctx, msg)
	}
	if err != nil {
		err := fmt.Errorf("error on gossiping the message to: %s, error: %v", addr, err)
		log.Error().Err(err).Send()
		return nil, err
	}
	return reply, nil
}

// sweeper looks for incoming message to the node's queue, and handles them concurrently
func (n *Node) sweeper() {
	for m := range n.queue {
		go concurrentHandler(n, m)
	}
}

// messageHandler is responsible to handle how to interpret incoming messages.
func (n *Node) messageHandler(e *entry) error {
	log := n.logger.With().
		Str("function", "messageHandler").
		Logger()

	///entry checking
	if e == nil || e.msg == nil {
		err := fmt.Errorf("entry is invalid")
		log.Error().Err(err).Send()
		return err
	}

	messageUUID := e.msg.Uuid
	invokeResp, err := n.regMngr.Invoke(e.ctx, e.msg.MessageType, e.msg.Payload)

	//if the tracker is not tracking the message, no further processing is needed
	if !n.tracker.ContainsID(messageUUID) {
		return err
	}

	if err != nil {
		err := fmt.Errorf("could not invoke: %v", err)
		log.Debug().Err(err).Send()
		n.tracker.FillMessageReply(messageUUID, nil, err)
		return err
	}

	n.tracker.FillMessageReply(messageUUID, invokeResp.Resp, invokeResp.Err)
	return nil
}

//TODO: Discuss logging errors in cases where the only option is to initialize a logger
//closeConnection closes a grpc client connection and prints the errors, if any,
func closeConnection(conn *grpc.ClientConn) {
	err := conn.Close()
	if err != nil {
		fmt.Printf("Error closing grpc connection %v", err)
	}
}

//concurrentHandler closes invokes messageHandler and prints the errors, if any,
func concurrentHandler(n *Node, e *entry) {
	log := n.logger.With().
		Str("function", "messageHandler").
		Logger()

	err := n.messageHandler(e)
	if err != nil {
		log.Error().Msgf("Error handling message: %v", err)
	}
}
