package gossip

import (
	"context"
	"net"
	"os"

	"github.com/golang/protobuf/proto"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/network"
	"github.com/dapperlabs/flow-go/network/gossip/cache"
	"github.com/dapperlabs/flow-go/network/gossip/order"
	"github.com/dapperlabs/flow-go/network/gossip/registry"
	"github.com/dapperlabs/flow-go/network/gossip/storage"
	"github.com/dapperlabs/flow-go/network/gossip/util"
	"github.com/dapperlabs/flow-go/network/mock"
	"github.com/dapperlabs/flow-go/protobuf/gossip/messages"
)

// To make sure that Node complies with the Service interface

const (
	hashProposal registry.MessageType = iota
	requestMessage
	sendEngine
)

// Node is holding the required information for a functioning gossip node
type Node struct {
	codec network.Codec // for encoding and decoding

	logger zerolog.Logger // for logging

	pt      PeersTable                // Peers table store mapping of ID and IP
	regMngr *registry.RegistryManager // manages the node's registry
	er      *EngineRegistry           // stores engines
	queue   chan *order.Order         // queue hosts incoming messages

	hashCache HashCache            // caches hashes of received messages
	msgStore  storage.MessageStore // stores incoming messages

	// QueueSize is the buffer size of the node for holding incoming Gossip Messages
	// Once buffer of a node gets full, it does not accept incoming messages

	id              flow.Identifier  // stores the local node identifier
	address         *messages.Socket // stores the address of the node
	peers           []string         // stores a list of other nodes' IPs
	fanoutSet       []string         // stores the fanout set of th node
	staticFanoutNum int              // stores the static fanout set of the node

	// server represents the protocol on which gnode will operate
	server ServePlacer
}

// NewNode takes a config type and extracts the required information
// for the node to function, in addition to options functions that
// indicate additional arguments to be specified for the node
//Todo CHANGE BEGIN 1138
func NewNode(opts ...Option) *Node {

	defaultAddress, _ := util.NewSocket("127.0.0.1:1773")
	defaultEngineRegistry, _ := NewEngineRegistry()

	n := &Node{
		codec:           &mock.Codec{},
		logger:          log.Output(zerolog.ConsoleWriter{Out: os.Stderr}).With().Caller().Logger(),
		regMngr:         registry.NewRegistryManager(nil),
		queue:           make(chan *order.Order, 10),
		hashCache:       cache.NewMemHashCache(),
		msgStore:        storage.NewMemMsgStore(),
		address:         defaultAddress,
		er:              defaultEngineRegistry,
		staticFanoutNum: 0,
	}

	for _, opt := range opts {
		opt(n)
	}

	n.initDefault()

	return n
}

// SetProtocol sets the underlying protocol for sending and
// receiving messages. The protocol should implement ServePlacer
func (n *Node) SetProtocol(s ServePlacer) {
	n.server = s
}

//Todo CHANGE ENd 1138

// hashProposal receives a hash of a message that is designated to be
// sent as a gossip, so that if it has not been received yet, a
// "requestMessage" is sent to the sender prompting him to send the
// original message hashMsgBytes: represents the hash of the message being proposed.
func (n *Node) hashProposal(ctx context.Context, hashMsgBytes []byte) ([]byte, error) {
	log := n.logger.With().
		Str("function", "hashProposal").
		Logger()

	// extract the information from the HashMessage
	hashBytes, senderAddr, err := util.ExtractHashMsgInfo(hashMsgBytes)
	if err != nil {
		err = errors.Wrap(err, "could not extract hash message info")
		log.Debug().Err(err).Send()
		return nil, err
	}

	// turn bytes of hash to a string
	hash := string(hashBytes)

	// if the message has been already received ignore the proposal
	if n.hashCache.IsReceived(hash) {
		return nil, nil
	}
	// set the message as confirmed
	n.hashCache.Confirm(hash)

	// this message will request the original message
	requestMsg, err := generateHashMessage(hashBytes, n.address)
	if err != nil {
		log.Debug().Err(err).Send()
		return nil, errors.Wrap(err, "could not generate hash message")
	}

	requestMsgBytes, err := proto.Marshal(requestMsg)
	if err != nil {
		log.Debug().Err(err).Send()
		return nil, errors.Wrap(err, "could not marshal message")
	}

	//slice with the address to send the message to
	address := []string{util.SocketToString(senderAddr)}

	// send a requestMessage to the sender to make him send the original message
	_, err = n.gossip(context.Background(), requestMsgBytes, address, requestMessage, ModeOneToOne)
	if err != nil {
		log.Debug().Err(err).Send()
		return nil, errors.Wrap(err, "could not gossip message")
	}

	return nil, nil
}

// requestMessage receives a hash of a message with the address of
// its sender, so that if the message exists in the store, it is
// sent to the requester. hashMsgBytes: represents the hash of the requested message.
func (n *Node) requestMessage(ctx context.Context, hashMsgBytes []byte) ([]byte, error) {
	log := n.logger.With().
		Str("function", "requestMessage").
		Logger()

	// extract information from hash message
	hashBytes, senderAddr, err := util.ExtractHashMsgInfo(hashMsgBytes)
	if err != nil {
		err = errors.Wrap(err, "could not extract hash message info")
		log.Debug().Err(err).Send()
		return nil, err
	}

	// turn bytes of hash into a string
	hash := string(hashBytes)

	// if the requested message does exist return an error
	if !n.hashCache.IsReceived(hash) {
		err = errors.New("no message of given hash is not found")
		log.Debug().Err(err).Send()
		return nil, err
	}

	// obtain message from store
	msg, err := n.msgStore.Get(hash)
	if err != nil {
		log.Debug().Err(err).Send()
		return nil, errors.Wrap(err, "could not get message from storage")
	}

	// slice with the address to send the message to
	address := []string{util.SocketToString(senderAddr)}

	// send the original message
	_, err = n.gossip(ctx, msg.GetPayload(), address, registry.MessageType(msg.GetMessageType()), ModeOneToOne)
	if err != nil {
		log.Debug().Err(err).Send()
		return nil, errors.Wrap(err, "could not gossip message")
	}

	return nil, nil
}

// sendEngine is the gossip HandleFunc that is responsible for delivering an
// event to the desired engine (will be called asynchronously with gossip)
func (n *Node) sendEngine(ctx context.Context, eprb []byte) ([]byte, error) {
	epr := &messages.EventProcessRequest{}

	if err := proto.Unmarshal(eprb, epr); err != nil {
		return nil, errors.Wrap(err, "could not unmarshal engine process request")
	}

	event, err := n.codec.Decode(epr.GetEvent())
	if err != nil {
		return nil, errors.Wrap(err, "could not decode event")
	}

	senderID := util.StringToID(epr.GetSenderID())
	return nil, n.er.Process(uint8(epr.GetChannelID()), senderID, event)
}

// Gossip synchronizes over the delivery
// i.e., sends a message to all recipients, and only blocks for
// delivery without blocking for their response
// payload:    represents the payload of the message
// recipients: the list of nodes designated to receive the message
// msgType:    represents the type of message to be gossiped
// 			   (e.g., the engine type, the method type to be invoked over the payload, etc)
func (n *Node) Gossip(ctx context.Context, payload []byte, recipients []string, msgType registry.MessageType) ([]*messages.GossipReply, error) {
	log := n.logger.With().
		Str("function", "Gossip").
		Logger()

	// generate GossipMessage type in order to hash it
	msg, err := generateGossipMessage(payload, recipients, uint64(msgType))
	if err != nil {
		err = errors.Wrap(err, "could not generate gossip message")
		log.Debug().Err(err).Send()
		return nil, err
	}

	_, _ = n.tryStore(msg)

	// hash the GossipMessage
	hash, err := util.ComputeHash(msg)
	if err != nil {
		err = errors.Wrap(err, "could not generate hash")
		log.Debug().Err(err).Send()
		return nil, err
	}

	// generate the HashMessage to be sent in the hash proposal
	hashMsg, err := generateHashMessage(hash, n.address)
	if err != nil {
		err = errors.Wrap(err, "could not generate hash message")
		log.Debug().Err(err).Send()
		return nil, err
	}

	// marshal HashMessage to bytes
	hashMsgBytes, err := proto.Marshal(hashMsg)
	if err != nil {
		err = errors.Wrap(err, "could not marshal message")
		log.Debug().Err(err).Send()
		return nil, err
	}

	var mode Mode
	switch len(recipients) {
	case 0:
		mode = ModeOneToAll
	case 1:
		mode = ModeOneToOne
	default:
		mode = ModeOneToMany
	}

	// send the hash proposal to recipients
	switch mode {
	case ModeOneToAll:
		_, err = n.gossip(ctx, hashMsgBytes, recipients, hashProposal, mode)
	case ModeOneToMany:
		_, err = n.gossip(ctx, payload, recipients, msgType, mode)
	case ModeOneToOne:
		_, err = n.gossip(ctx, payload, recipients, msgType, mode)
	default:
		return nil, errors.Errorf("unknown gossip mode: (%v)", mode)
	}

	if err != nil {
		err = errors.Wrap(err, "could not gossip message")
		log.Debug().Err(err).Send()
		return nil, err
	}

	return nil, nil
}

// QueueService is invoked remotely using the gRPC stub,
// it receives a message from a remote node and places it inside the local
// nodes queue it is synchronized with the remote node on the message reception
// (and NOT reply), i.e., blocks the remote node until either a timeout or
// placement of the message into the queue msg: the message to be placed in the queue
func (n *Node) QueueService(ctx context.Context, msg *messages.GossipMessage) (*messages.GossipReply, error) {
	log := n.logger.With().
		Str("function", "QueuesService").
		Logger()
	//if context timed out already, return an error
	select {
	case <-ctx.Done():
		log.Debug().Err(ErrTimedOut).Send()
		return &messages.GossipReply{}, ErrTimedOut
	default:
	}

	msgExists, err := n.tryStore(msg)

	if err != nil {
		err = errors.Wrap(err, "error in storing message")
		log.Debug().Err(err).Send()
		return &messages.GossipReply{}, err
	}

	// if message was confirmed, then ignore it
	if msgExists {
		return &messages.GossipReply{}, nil
	}

	// no recipient for a message means that the message is a gossip to ALL so the node also broadcasts that message

	switch len(msg.GetRecipients()) {
	// If the message is One To All, then forward it as well
	case 0:
		// In case the recipients list was empty, then this message is one
		// to all.
		go func() {
			if _, err := n.gossip(ctx, msg.GetPayload(), nil, registry.MessageType(msg.GetMessageType()), ModeOneToAll); err != nil {
				n.logger.Debug().Err(err).Send()
			}
		}()
	default:
		// Do nothing as this connection is one to one or one to many. None of them
		// uses forwarding
	}

	// Wait until the context expires, or if the msg got placed on the queue
	select {
	case <-ctx.Done():
		log.Debug().Err(ErrTimedOut).Send()
		return &messages.GossipReply{}, ErrTimedOut
	case n.queue <- order.NewAsync(ctx, msg):
		return &messages.GossipReply{}, nil
	}

}

// Serve starts an async node grpc server, and its sweeper as well
func (n *Node) Serve(listener net.Listener) error {
	// Send of the sweeper to monitor the queue
	go n.sweeper()
	// TODO: make serve return error
	n.server.Serve(listener)

	return nil
}

// initDefault adds the default functions to the registry of the node
func (n *Node) initDefault() {
	err := n.pickGossipPartners()
	if err != nil {
		n.logger.Debug().Err(err).Send()
	}

	if err != nil {
		n.logger.Debug().Err(err).Send()
	}

	// place functions and their keys into arrays.
	msgTypes := []registry.MessageType{hashProposal, requestMessage, sendEngine}
	f := []registry.HandleFunc{n.hashProposal, n.requestMessage, n.sendEngine}

	err = n.regMngr.AddDefaultTypes(msgTypes, f)

	if err != nil {
		n.logger.Debug().Err(err).Send()
	}
}

// RegisterFunc allows the addition of new message types to the node's registry
// msgType:   type of message to be registered
// handlFunc: the signature of the registered function
func (n *Node) RegisterFunc(msgType registry.MessageType, f registry.HandleFunc) error {
	return n.regMngr.AddMessageType(msgType, f, false)
}

// gossip is the main function through which messages of all types are gossiped
// payload:       represents the payload of the function
// recipients:    the list of recipients of the gossip message, a nil represents
// 				  a gossip to all the network
// msgType:       represents the type of the message being gossiped
// isSynchronous: the type of gossiping (Sync,Async)
func (n *Node) gossip(ctx context.Context, payload []byte, recipients []string, msgType registry.MessageType, mode Mode) ([]*messages.GossipReply, error) {
	log := n.logger.With().
		Str("function", "gossip").
		Logger()
	// casting gossip message attributes into a protobuf message
	gossipMsg, err := generateGossipMessage(payload, recipients, uint64(msgType))
	if err != nil {
		err := errors.Wrap(err, "could not generate gossip message")
		log.Error().Err(err).Send()
		return nil, err
	}

	// make sure message is in storage before gossiping
	_, err = n.tryStore(gossipMsg)

	if err != nil {
		err := errors.Wrap(err, "error in storing message")
		log.Error().Err(err).Send()
		return nil, err
	}

	actualRecipients := recipients

	// If the recipients list is empty, this signals oneToAll gossip mode,
	// so we use the fanout set as recipients
	switch mode {
	case ModeOneToAll:
		// recipient list is empty, hence this is a gossip to all (i.e., broadcast)
		gossipMsg.Recipients = nil
		actualRecipients = n.fanoutSet
	case ModeOneToMany:
		// don't need to change anything for one-to-many
	case ModeOneToOne:
		// don't need to change anything for one-to-one
	default:
	}

	numberOfRecipients := len(actualRecipients)

	gossipErrChan := make(chan error)
	gossipRepliesChan := make(chan *messages.GossipReply)
	gossipErr := &gossipError{}
	gossipReplies := make([]*messages.GossipReply, numberOfRecipients)

	// TODO for v1.2
	// Note: The order of gossip replies and errors is not defined. Perhaps
	// this is something we need to define for later, as currently there is no clear way
	// to match errors to responses.
	// Loop through recipients
	for _, addr := range actualRecipients {
		go func(addr string) {
			reply, err := n.placeMessage(context.Background(), addr, gossipMsg, mode)
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

// placeMessage is invoked by the node, and encapsulates the process of placing
// a single message to a recipient's incoming queue placeMessage takes a context,
// address of target node, the message to send, and whether to use sync or async
// mode (true for sync, false for async) placeMessage establishes and manages a
// gRPC client inside
func (n *Node) placeMessage(ctx context.Context, addr string, msg *messages.GossipMessage, mode Mode) (*messages.GossipReply, error) {
	return n.server.Place(ctx, addr, msg, mode)
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
		err := errors.New("order provided is not valid")
		log.Error().Err(err).Send()
		return err
	}

	invokeResp, err := n.regMngr.Invoke(context.Background(), registry.MessageType(ord.Msg.GetMessageType()), ord.Msg.GetPayload())
	if err != nil {
		ord.Fill(nil, err)
		err := errors.Wrap(err, "could not invoke")
		log.Debug().Err(err).Send()
		return err
	}

	ord.Fill(invokeResp.Resp, invokeResp.Err)

	return nil
}

// concurrentHandler handles the received messages by gnode in a concurrent manner.
// n: node whose messages are to be handled
// o: order is the order instance managing the message of the node
func concurrentHandler(n *Node, o *order.Order) {
	log := n.logger.With().
		Str("function", "concurrentHandler").
		Logger()
	err := n.messageHandler(o)
	if err != nil {
		err := errors.Wrap(err, "error handling message")
		log.Error().Err(err).Send()
	}
}

// tryStore receives a gossip message and checks if its present inside the node storage.
// in case it exists it return true, otherwise it adds the entry to the data base and returns false.
// msg: message to be stored
func (n *Node) tryStore(msg *messages.GossipMessage) (bool, error) {
	hashBytes, err := util.ComputeHash(msg)
	if err != nil {
		return false, errors.Wrap(err, "could not compute hash")
	}

	hash := string(hashBytes)

	// add message to storage and set it as received
	// if message is a duplicate then ignore it
	if n.hashCache.Receive(hash) {
		return true, nil
	}

	err = n.msgStore.Put(hash, msg)
	if err != nil {
		return false, errors.Wrap(err, "could not store message")
	}
	return false, nil
}

// Register stores an engine into the engine registry and returns a conduit for
// that engine
func (n *Node) Register(channelID uint8, engine network.Engine) (network.Conduit, error) {
	if err := n.er.Add(channelID, engine); err != nil {
		return nil, errors.Wrap(err, "could not add engnie to registry")
	}

	return &conduit{send: func(event interface{}, targetIDs ...flow.Identifier) error {
		return n.send(channelID, event, targetIDs...)
	}}, nil
}

// send is the basic conduit function that gets wrapped for every engine
// registered
func (n *Node) send(channelID uint8, event interface{}, targetIDs ...flow.Identifier) error {
	eventBytes, err := n.codec.Encode(event)
	if err != nil {
		return errors.Wrap(err, "could not encode event")
	}

	sender := util.IDToString(n.id)
	eprBytes, err := proto.Marshal(&messages.EventProcessRequest{ChannelID: uint32(channelID), Event: eventBytes, SenderID: sender})
	if err != nil {
		return errors.Wrap(err, "could not marshal event process request")
	}

	targetIPs, err := n.pt.GetIPs(targetIDs...)
	if err != nil {
		return errors.Wrap(err, "could not get IPs of target nodes")
	}

	_, err = n.Gossip(context.Background(), eprBytes, targetIPs, sendEngine)
	if err != nil {
		return errors.Wrap(err, "could not gossip event process request")
	}

	return nil
}

// pickGossipPartners randomly chooses setSize gossip partners from a given set of peers
func (n *Node) pickGossipPartners() error {
	if len(n.peers) == 0 {
		return errors.New("peers list is empty")
	}

	gossipPartners, err := util.RandomSubSet(n.peers, n.staticFanoutNum)
	if err != nil {
		return errors.Wrap(err, "could not pick gossip partners")
	}

	n.fanoutSet = gossipPartners

	return nil
}
