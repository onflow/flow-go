package gnode

import (
	"context"
	"fmt"
	"net"
	"os"

	"github.com/rs/zerolog/log"

	"github.com/golang/protobuf/proto"
	"github.com/rs/zerolog"
	"google.golang.org/grpc"

	"github.com/dapperlabs/flow-go/pkg/grpc/shared"
	"github.com/dapperlabs/flow-go/pkg/network/gossip"
	"github.com/dapperlabs/flow-go/pkg/network/gossip/v1/order"
)

// To make sure that Node complies with the gossip.Service interface
var _ gossip.Service = (*Node)(nil)

const DefaultQueueSize = 10

// Node is holding the required information for a functioning async gossip node
type Node struct {
	logger  zerolog.Logger
	regMngr *registryManager
	queue   chan *order.Order

	hashCache hashCache
	msgStore  messageDatabase

	// queueSize is the buffer size of the node for holding incoming gossip messages.
	// Once buffer of a node gets full, it does not accept incoming messages
	queueSize int

	address         string
	peers           []string
	fanoutSet       []string
	staticFanoutNum int
}

// NewNode returns a new instance of gnode with a specified registry, static fanout set size, and queue size.
func NewNode(msgTypesRegistry Registry, addr string, peers []string, staticFanoutNum int, queueSize int) *Node {
	//setting default queue size of the node to 10:
	if queueSize <= 0 {
		queueSize = DefaultQueueSize
	}

	node := &Node{
		logger:          log.Output(zerolog.ConsoleWriter{Out: os.Stderr}).With().Caller().Logger(),
		regMngr:         newRegistryManager(msgTypesRegistry),
		queueSize:       queueSize,
		queue:           make(chan *order.Order, queueSize),
		hashCache:       newMemoryHashCache(),
		msgStore:        newMemMsgDatabase(),
		address:         addr,
		peers:           peers,
		staticFanoutNum: staticFanoutNum,
	}

	err := node.initDefault()
	if err != nil {
		log := (*node).logger.With().
			Str("function", "NewNode").Logger()
		log.Debug().Err(ErrInternal).Send()
	}

	return node
}

// syncHashProposal receives a hash of a message that is meant to be sent as SyncGossip, so
// that in case it has not been received, the function returns a non-nil value prompting the sender
// to send the original message
//Todo: This function is planned to be upgraded to a streaming version so that no return value is required upon a negative answer
func (n *Node) syncHashProposal(ctx context.Context, hashMsgBytes []byte) ([]byte, error) {
	// extract the info (hash) from the HashMessage
	hashBytes, _, err := extractHashMsgInfo(hashMsgBytes)
	if err != nil {
		return nil, fmt.Errorf("could not extract hash message info: %v", err)
	}

	// cast bytes into string
	hash := string(hashBytes)

	// if the message has been already received ignore the proposal
	if n.hashCache.isReceived(hash) {
		return nil, nil
	}

	// cashing the hash as a confirmed one
	n.hashCache.confirm(hash)

	// returning the address of this node so that the sender sends the original message
	return []byte(n.address), nil
}

// asyncHashProposal receives a hash of a message that is meant to be sent as an AsyncGossip, so that
// if it has not been received yet, a "requestMessage" is sent to the sender prompting him to send the original message
func (n *Node) asyncHashProposal(ctx context.Context, hashMsgBytes []byte) ([]byte, error) {
	// extract the info (hash) from the HashMessage
	hashBytes, senderAddr, err := extractHashMsgInfo(hashMsgBytes)
	if err != nil {
		return nil, fmt.Errorf("could not extract hash message info: %v", err)
	}

	// turn bytes of hash to a string
	hash := string(hashBytes)

	// if the message has been already recieved ignore the proposal
	if n.hashCache.isReceived(hash) {
		return nil, nil
	}
	// set the message as confirmed
	n.hashCache.confirm(hash)

	// async sender does not wait for the response to the hash proposal, hence, we need to prepare a request message
	// if we are interested in the original content of the proposed hash
	// this message will request the original message
	requestMsg, err := generateHashMessage(hashBytes, n.address)
	if err != nil {
		return nil, fmt.Errorf("could not generate hash message: %v", err)
	}

	requestMsgBytes, err := proto.Marshal(requestMsg)
	if err != nil {
		return nil, fmt.Errorf("could not marshal message: %v", err)
	}

	// send a requestMessage to the sender to ask the sender send the original message
	_, err = n.gossip(context.Background(), requestMsgBytes, []string{senderAddr}, "requestMessage", false)
	if err != nil {
		return nil, fmt.Errorf("could not gossip message: %v", err)
	}

	return nil, nil
}

// requestMessage receives the hash of a message with the address of its sender as a result of a hash proposal,
// so that if the message exists in the data store, it is sent to the requester.
func (n *Node) requestMessage(ctx context.Context, hashMsgBytes []byte) ([]byte, error) {
	// extract the info (hash) from the HashMessage
	hashBytes, senderAddr, err := extractHashMsgInfo(hashMsgBytes)
	if err != nil {
		return nil, fmt.Errorf("could not extract hash message info: %v", err)
	}

	// turn bytes of hash into a string
	hash := string(hashBytes)

	// in case the message requested does not exist return an error
	if !n.hashCache.isReceived(hash) {
		return nil, fmt.Errorf("message of hash %v not found in storage", hash)
	}

	// obtain message from store
	msg, err := n.msgStore.Get(hash)
	if err != nil {
		return nil, fmt.Errorf("could not get message from database: %v", err)
	}

	// send the original message
	_, err = n.gossip(ctx, msg.GetPayload(), []string{senderAddr}, msg.GetMessageType(), false)
	if err != nil {
		return nil, fmt.Errorf("could not gossip message: %v", err)
	}

	return nil, nil

}

// SyncGossip synchronizes over the reply of recipients
// i.e., it sends a message to all recipients and blocks for their reply
func (n *Node) SyncGossip(ctx context.Context, payload []byte, recipients []string, msgType string) ([]*shared.GossipReply, error) {

	// generate GossipMessage type in order to hash it
	msg, err := generateGossipMessage(payload, recipients, msgType)
	if err != nil {
		return []*shared.GossipReply{}, fmt.Errorf("could not generate gossip message: %v", err)
	}

	// make sure message is stored in the database
	_, err = n.tryStore(msg)

	// hash the GossipMessage
	hash, err := computeHash(msg)
	if err != nil {
		return []*shared.GossipReply{}, fmt.Errorf("could not generate hash: %v", err)
	}

	// generate the HashMessage to be sent in the hash proposal
	hashMsg, err := generateHashMessage(hash, n.address)
	if err != nil {
		return []*shared.GossipReply{}, fmt.Errorf("could not generate hash message: %v", err)
	}

	// marshal HashMessage to bytes
	hashMsgBytes, err := proto.Marshal(hashMsg)
	if err != nil {
		return []*shared.GossipReply{}, fmt.Errorf("could not marshal message: %v", err)
	}

	/*
		Part I: sending individual one-to-one direct hash proposals.
		Note: hash proposals in synchronous mode cannot be gossiped within the overlay since the proposer should be blocked till
		a response
	*/

	// realRecipients will store the addresses of nodes who want the actual message
	realRecipients := make([]string, 0)

	// send sync hash proposals
	replies, err := n.gossip(ctx, hashMsgBytes, recipients, "syncHashProposal", true)

	// iterate over replies, if the reply is non-nil, this means it is equal to the address of the recipient
	// and that the recipient wants the original message. Hence add his address to realRecipients
	for _, rep := range replies {
		if rep != nil && rep.ResponseByte != nil {
			realRecipients = append(realRecipients, string(rep.ResponseByte))
		}
	}

	/*
		Part II: send actual message to addresses who have requested by positively responding to the hash proposal
	*/
	return n.gossip(ctx, payload, realRecipients, msgType, true)
}

// AsyncGossip synchronizes over the delivery
// i.e., sends a message to all recipients, and only blocks for delivery without blocking for their response
func (n *Node) AsyncGossip(ctx context.Context, payload []byte, recipients []string, msgType string) ([]*shared.GossipReply, error) {
	// generate GossipMessage type in order to hash it
	msg, err := generateGossipMessage(payload, recipients, msgType)
	if err != nil {
		return []*shared.GossipReply{}, fmt.Errorf("could not generate gossip message: %v", err)
	}
	_, err = n.tryStore(msg)

	// hashing the GossipMessage
	hash, err := computeHash(msg)
	if err != nil {
		return []*shared.GossipReply{}, fmt.Errorf("could not generate hash: %v", err)
	}

	// generating the HashMessage to be sent in the hash proposal
	hashMsg, err := generateHashMessage(hash, n.address)
	if err != nil {
		return []*shared.GossipReply{}, fmt.Errorf("could not generate hash message: %v", err)
	}

	// marshaling HashMessage to bytes
	hashMsgBytes, err := proto.Marshal(hashMsg)
	if err != nil {
		return []*shared.GossipReply{}, fmt.Errorf("could not marshal message: %v", err)
	}

	// sending the hash proposal to recipients
	_, err = n.gossip(ctx, hashMsgBytes, recipients, "asyncHashProposal", false)
	if err != nil {
		return nil, fmt.Errorf("could not gossip message: %v", err)
	}

	return []*shared.GossipReply{}, nil
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

	msgExists, err := n.tryStore(msg)

	if err != nil {
		return &shared.GossipReply{}, fmt.Errorf("error in storing message: %v", err)
	}

	// if message already has been received discard it
	if msgExists {
		return &shared.GossipReply{}, nil
	}
	// no recipient for a message means that the message is a gossip to ALL so the node also broadcasts that message
	if len(msg.GetRecipients()) == 0 {
		go func(ctx context.Context, msg *shared.GossipMessage) {
			n.gossip(context.Background(), msg.GetPayload(), nil, msg.GetMessageType(), false)
		}(ctx, msg)
	}

	// Wait until the context expires, or if the msg got placed on the queue
	select {
	case <-ctx.Done():
		log.Debug().Err(ErrTimedOut).Send()
		return &shared.GossipReply{}, ErrTimedOut
	case n.queue <- order.NewOrder(ctx, msg, false):
		return &shared.GossipReply{}, nil
	}
}

// SyncQueue is invoked remotely using the gRPC stub,
// it receives a message from a remote node and places it inside the local nodes queue
// it is synchronized with the remote node on the message reply, i.e., blocks the remote node until either
// a timeout or a reply is getting prepared
func (n *Node) SyncQueue(ctx context.Context, msg *shared.GossipMessage) (*shared.GossipReply, error) {
	ord := order.NewOrder(ctx, msg, true)

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

	// check if message is stored, and store it if it was not stored
	msgExists, err := n.tryStore(msg)
	if err != nil {
		return &shared.GossipReply{}, fmt.Errorf("error in storing message: %v", err)
	}
	// if the message was already confirmed
	if msgExists {
		return &shared.GossipReply{}, nil
	}

	// no recipient for a message means that the message is a gossip to ALL so the node also broadcasts that message
	if len(msg.GetRecipients()) == 0 {
		go func(ctx context.Context, msg *shared.GossipMessage) {
			n.gossip(ctx, msg.GetPayload(), nil, msg.GetMessageType(), false)
		}(ctx, msg)
	}

	/*
		Part I: blocking on reception
	*/
	//getting blocked until either a timeout,
	// or the message getting placed into the local nodes queue
	select {
	case <-ctx.Done():
		log.Debug().Err(ErrTimedOut).Send()
		return &shared.GossipReply{}, ErrTimedOut
	case n.queue <- ord:
	}

	/*
		Part II: blocking on processing
	*/
	// getting blocked until either a timeout
	// or the message getting processed out of the queue of the local node
	select {
	case <-ctx.Done():
		log.Debug().Err(ErrTimedOut).Send()
		return &shared.GossipReply{}, ErrTimedOut
	case <-ord.Done():
	}
	msgReply, msgErr := ord.Result()
	if msgErr != nil {
		return &shared.GossipReply{}, msgErr
	}
	return &shared.GossipReply{ResponseByte: msgReply}, nil
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
		err := fmt.Errorf("failed to serve: %v", err)
		log.Panic().Err(err).Send()
		return err
	}

	return nil
}

// initDefault adds the default functions to the registry of the node
func (n *Node) initDefault() error {
	err := n.pickGossipPartners()
	if err != nil {
		return err
	}

	err = n.RegisterFunc("syncHashProposal", n.syncHashProposal)
	if err != nil {
		return err
	}

	err = n.RegisterFunc("asyncHashProposal", n.asyncHashProposal)
	if err != nil {
		return err
	}

	err = n.RegisterFunc("requestMessage", n.requestMessage)
	if err != nil {
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

	// make sure message is in storage before gossiping
	_, err = n.tryStore(gossipMsg)

	if err != nil {
		return nil, fmt.Errorf("error in storing message: %v", err)
	}

	numberOfRecipients := len(recipients)
	actualRecipients := recipients
	if numberOfRecipients == 0 {
		// if recipient list is empty it means that the gossip message is meant for all the nodes, i.e., One-To-All
		// in this case, the gossip is going over the gossip partners of the node
		numberOfRecipients = len(n.fanoutSet)
		actualRecipients = n.fanoutSet
	}

	gossipErrChan := make(chan error)
	gossipRepliesChan := make(chan *shared.GossipReply)
	gossipErr := &gossipError{}
	gossipReplies := make([]*shared.GossipReply, numberOfRecipients)

	// TODO for v1.2
	// Note: The order of gossip replies and errors is not defined. Perhaps
	// this is something we need to define for later, as currently there is no clear way
	// to match errors to responses.
	// Loop through recipients

	for _, addr := range actualRecipients {
		go func(addr string) {
			reply, err := n.placeMessage(ctx, addr, gossipMsg, isSynchronous)
			gossipErrChan <- err
			gossipRepliesChan <- reply
		}(addr)
	}

	// Append errors and replies from the respective channels
	for i := 0; i < numberOfRecipients; i++ {
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
func (n *Node) messageHandler(ord *order.Order) error {
	log := n.logger.With().
		Str("function", "messageHandler").
		Logger()
	if !order.Valid(ord) {
		log.Error().Err(fmt.Errorf("receieved invalid order")).Send()
		return fmt.Errorf("order provided is not valid %v", ord)
	}

	invokeResp, err := n.regMngr.Invoke(ord.Ctx, ord.Msg.GetMessageType(), ord.Msg.GetPayload())
	if err != nil {
		ord.Fill(nil, err)
		log.Debug().Err(err).Send()
		return fmt.Errorf("could not invoke: %v", err)
	}

	ord.Fill(invokeResp.Resp, invokeResp.Err)

	return nil
}

//concurrentHandler invokes messageHandler and prints the errors, if any,
func concurrentHandler(n *Node, o *order.Order) {
	err := n.messageHandler(o)
	if err != nil {
		fmt.Printf("Error handling message: %v\n", err)
	}
}

// tryStore receives a gossip message and checks if it exits inside the node database.
// in case it exists it return true, otherwise it adds the entry to the data base and returns false.
func (n *Node) tryStore(msg *shared.GossipMessage) (bool, error) {
	hashBytes, err := computeHash(msg)
	if err != nil {
		return false, fmt.Errorf("could not compute hash: %v", err)
	}

	hash := string(hashBytes)

	// if message already received then ignore it.
	if n.hashCache.isReceived(hash) {
		return true, nil
	}

	// add message to storage and set it as received
	n.hashCache.receive(hash)
	err = n.msgStore.Put(hash, msg)
	if err != nil {
		return false, fmt.Errorf("could not store message: %v", err)
	}
	return false, nil
}

// pickGossipPartners randomly chooses setSize gossip partners from a given set of peers
func (n *Node) pickGossipPartners() error {
	if len(n.peers) == 0 {
		return fmt.Errorf("peers list is empty")
	}

	// if number of given peers is less than the fanout set size, then return these peers
	if len(n.peers) < n.staticFanoutNum {
		n.fanoutSet = n.peers
		return nil
	}

	var (
		gossipPartners     = make([]string, n.staticFanoutNum)
		us                 = newUniqSelector(len(n.peers))
		index          int = 0
		err            error
	)

	for i := 0; i < n.staticFanoutNum; i++ {
		index, err = us.Int()
		if err != nil {
			return fmt.Errorf("could not pick index for a gossip partner: %v", err)
		}
		if index >= len(n.peers) {
			return fmt.Errorf("UniqSelector goes beyond peers size: %d", index)
		}
		gossipPartners[i] = n.peers[index]
	}

	n.fanoutSet = gossipPartners

	return nil
}
