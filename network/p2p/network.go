package p2p

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/ipfs/go-datastore"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/crypto/hash"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/network"
	netcache "github.com/onflow/flow-go/network/cache"
	"github.com/onflow/flow-go/network/channels"
	"github.com/onflow/flow-go/network/message"
	"github.com/onflow/flow-go/network/p2p/conduit"
	"github.com/onflow/flow-go/network/queue"
	_ "github.com/onflow/flow-go/utils/binstat"
	"github.com/onflow/flow-go/utils/logging"
)

const (
	// DefaultReceiveCacheSize represents size of receive cache that keeps hash of incoming messages
	// for sake of deduplication.
	DefaultReceiveCacheSize = 10e4
	// eventIDPackingPrefix is used as a salt to generate payload hash for messages.
	eventIDPackingPrefix = "libp2ppacking"
)

// NotEjectedFilter is an identity filter that, when applied to the identity
// table at a given snapshot, returns all nodes that we should communicate with
// over the networking layer.
//
// NOTE: The protocol state includes nodes from the previous/next epoch that should
// be included in network communication. We omit any nodes that have been ejected.
var NotEjectedFilter = filter.Not(filter.Ejected)

type NetworkOptFunction func(*Network)

func WithConduitFactory(f network.ConduitFactory) NetworkOptFunction {
	return func(n *Network) {
		n.conduitFactory = f
	}
}

// Network represents the overlay network of our peer-to-peer network, including
// the protocols for handshakes, authentication, gossiping and heartbeats.
type Network struct {
	sync.RWMutex
	*component.ComponentManager
	identityProvider            module.IdentityProvider
	logger                      zerolog.Logger
	codec                       network.Codec
	me                          module.Local
	mw                          network.Middleware
	metrics                     module.NetworkMetrics
	receiveCache                *netcache.ReceiveCache // used to deduplicate incoming messages
	queue                       network.MessageQueue
	subscriptionManager         network.SubscriptionManager // used to keep track of subscribed channels
	conduitFactory              network.ConduitFactory
	topology                    network.Topology
	registerEngineRequests      chan *registerEngineRequest
	registerBlobServiceRequests chan *registerBlobServiceRequest
}

var _ network.Network = &Network{}
var _ network.Overlay = &Network{}

type registerEngineRequest struct {
	channel          channels.Channel
	messageProcessor network.MessageProcessor
	respChan         chan *registerEngineResp
}

type registerEngineResp struct {
	conduit network.Conduit
	err     error
}

type registerBlobServiceRequest struct {
	channel  channels.Channel
	ds       datastore.Batching
	opts     []network.BlobServiceOption
	respChan chan *registerBlobServiceResp
}

type registerBlobServiceResp struct {
	blobService network.BlobService
	err         error
}

var ErrNetworkShutdown = errors.New("network has already shutdown")

type NetworkParameters struct {
	Logger              zerolog.Logger
	Codec               network.Codec
	Me                  module.Local
	MiddlewareFactory   func() (network.Middleware, error)
	Topology            network.Topology
	SubscriptionManager network.SubscriptionManager
	Metrics             module.NetworkMetrics
	IdentityProvider    module.IdentityProvider
	ReceiveCache        *netcache.ReceiveCache
	Options             []NetworkOptFunction
}

// NewNetwork creates a new naive overlay network, using the given middleware to
// communicate to direct peers, using the given codec for serialization, and
// using the given state & cache interfaces to track volatile information.
// csize determines the size of the cache dedicated to keep track of received messages
func NewNetwork(param *NetworkParameters) (*Network, error) {

	mw, err := param.MiddlewareFactory()
	if err != nil {
		return nil, fmt.Errorf("could not create middleware: %w", err)
	}

	n := &Network{
		logger:                      param.Logger,
		codec:                       param.Codec,
		me:                          param.Me,
		mw:                          mw,
		receiveCache:                param.ReceiveCache,
		topology:                    param.Topology,
		metrics:                     param.Metrics,
		subscriptionManager:         param.SubscriptionManager,
		identityProvider:            param.IdentityProvider,
		conduitFactory:              conduit.NewDefaultConduitFactory(),
		registerEngineRequests:      make(chan *registerEngineRequest),
		registerBlobServiceRequests: make(chan *registerBlobServiceRequest),
	}

	for _, opt := range param.Options {
		opt(n)
	}

	n.mw.SetOverlay(n)

	if err := n.conduitFactory.RegisterAdapter(n); err != nil {
		return nil, fmt.Errorf("could not register network adapter: %w", err)
	}

	n.ComponentManager = component.NewComponentManagerBuilder().
		AddWorker(n.runMiddleware).
		AddWorker(n.processRegisterEngineRequests).
		AddWorker(n.processRegisterBlobServiceRequests).Build()

	return n, nil
}

func (n *Network) processRegisterEngineRequests(parent irrecoverable.SignalerContext, ready component.ReadyFunc) {
	<-n.mw.Ready()
	ready()

	for {
		select {
		case req := <-n.registerEngineRequests:
			conduit, err := n.handleRegisterEngineRequest(parent, req.channel, req.messageProcessor)
			resp := &registerEngineResp{
				conduit: conduit,
				err:     err,
			}

			select {
			case <-parent.Done():
				return
			case req.respChan <- resp:
			}
		case <-parent.Done():
			return
		}
	}
}

func (n *Network) processRegisterBlobServiceRequests(parent irrecoverable.SignalerContext, ready component.ReadyFunc) {
	<-n.mw.Ready()
	ready()

	for {
		select {
		case req := <-n.registerBlobServiceRequests:
			blobService, err := n.handleRegisterBlobServiceRequest(parent, req.channel, req.ds, req.opts)
			resp := &registerBlobServiceResp{
				blobService: blobService,
				err:         err,
			}

			select {
			case <-parent.Done():
				return
			case req.respChan <- resp:
			}
		case <-parent.Done():
			return
		}
	}
}

func (n *Network) runMiddleware(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
	// setup the message queue
	// create priority queue
	n.queue = queue.NewMessageQueue(ctx, queue.GetEventPriority, n.metrics)

	// create workers to read from the queue and call queueSubmitFunc
	queue.CreateQueueWorkers(ctx, queue.DefaultNumWorkers, n.queue, n.queueSubmitFunc)

	n.mw.Start(ctx)
	<-n.mw.Ready()

	ready()

	<-n.mw.Done()
}

func (n *Network) handleRegisterEngineRequest(parent irrecoverable.SignalerContext, channel channels.Channel, engine network.MessageProcessor) (network.Conduit, error) {
	if !channels.ChannelExists(channel) {
		return nil, fmt.Errorf("unknown channel: %s, should be registered in topic map", channel)
	}

	err := n.subscriptionManager.Register(channel, engine)
	if err != nil {
		return nil, fmt.Errorf("failed to register engine for channel %s: %w", channel, err)
	}

	n.logger.Info().
		Str("channel_id", channel.String()).
		Msg("channel successfully registered")

	// create the conduit
	newConduit, err := n.conduitFactory.NewConduit(parent, channel)
	if err != nil {
		return nil, fmt.Errorf("could not create conduit using factory: %w", err)
	}

	return newConduit, nil
}

func (n *Network) handleRegisterBlobServiceRequest(parent irrecoverable.SignalerContext, channel channels.Channel, ds datastore.Batching, opts []network.BlobServiceOption) (network.BlobService, error) {
	bs := n.mw.NewBlobService(channel, ds, opts...)

	// start the blob service using the network's context
	bs.Start(parent)

	return bs, nil
}

// Register will register the given engine with the given unique engine engineID,
// returning a conduit to directly submit messages to the message bus of the
// engine.
func (n *Network) Register(channel channels.Channel, messageProcessor network.MessageProcessor) (network.Conduit, error) {
	respChan := make(chan *registerEngineResp)

	select {
	case <-n.ComponentManager.ShutdownSignal():
		return nil, ErrNetworkShutdown
	case n.registerEngineRequests <- &registerEngineRequest{
		channel:          channel,
		messageProcessor: messageProcessor,
		respChan:         respChan,
	}:
		select {
		case <-n.ComponentManager.ShutdownSignal():
			return nil, ErrNetworkShutdown
		case resp := <-respChan:
			return resp.conduit, resp.err
		}
	}
}

func (n *Network) RegisterPingService(pingProtocol protocol.ID, provider network.PingInfoProvider) (network.PingService, error) {
	select {
	case <-n.ComponentManager.ShutdownSignal():
		return nil, ErrNetworkShutdown
	default:
		return n.mw.NewPingService(pingProtocol, provider), nil
	}
}

// RegisterBlobService registers a BlobService on the given channel.
// The returned BlobService can be used to request blobs from the network.
func (n *Network) RegisterBlobService(channel channels.Channel, ds datastore.Batching, opts ...network.BlobServiceOption) (network.BlobService, error) {
	respChan := make(chan *registerBlobServiceResp)

	select {
	case <-n.ComponentManager.ShutdownSignal():
		return nil, ErrNetworkShutdown
	case n.registerBlobServiceRequests <- &registerBlobServiceRequest{
		channel:  channel,
		ds:       ds,
		opts:     opts,
		respChan: respChan,
	}:
		select {
		case <-n.ComponentManager.ShutdownSignal():
			return nil, ErrNetworkShutdown
		case resp := <-respChan:
			return resp.blobService, resp.err
		}
	}
}

// UnRegisterChannel unregisters the engine for the specified channel. The engine will no longer be able to send or
// receive messages from that channel.
func (n *Network) UnRegisterChannel(channel channels.Channel) error {
	err := n.subscriptionManager.Unregister(channel)
	if err != nil {
		return fmt.Errorf("failed to unregister engine for channel %s: %w", channel, err)
	}
	return nil
}

func (n *Network) Identities() flow.IdentityList {
	return n.identityProvider.Identities(NotEjectedFilter)
}

func (n *Network) Identity(pid peer.ID) (*flow.Identity, bool) {
	return n.identityProvider.ByPeerID(pid)
}

func (n *Network) Receive(nodeID flow.Identifier, msg *message.Message, decodedMsgPayload interface{}) error {
	err := n.processNetworkMessage(nodeID, msg, decodedMsgPayload)
	if err != nil {
		return fmt.Errorf("could not process message: %w", err)
	}
	return nil
}

func (n *Network) processNetworkMessage(senderID flow.Identifier, message *message.Message, decodedMsgPayload interface{}) error {
	// checks the cache for deduplication and adds the message if not already present
	if !n.receiveCache.Add(message.EventID) {
		// drops duplicate message
		n.logger.Debug().
			Hex("sender_id", senderID[:]).
			Hex("event_id", message.EventID).
			Str("channel", message.ChannelID).
			Msg("dropping message due to duplication")

		n.metrics.NetworkDuplicateMessagesDropped(message.ChannelID, message.Type)

		return nil
	}

	// create queue message
	qm := queue.QMessage{
		Payload:  decodedMsgPayload,
		Size:     message.Size(),
		Target:   channels.Channel(message.ChannelID),
		SenderID: senderID,
	}

	// insert the message in the queue
	err := n.queue.Insert(qm)
	if err != nil {
		return fmt.Errorf("failed to insert message in queue: %w", err)
	}
	n.logger.Debug().
		Str("channel", message.ChannelID).
		Str("payload_type", fmt.Sprintf("%T", decodedMsgPayload)).
		Str("sender_id", senderID.String()).
		Msg("received message inserted in queue")

	return nil
}

// genNetworkMessage uses the codec to encode an event into a NetworkMessage
func (n *Network) genNetworkMessage(channel channels.Channel, event interface{}, targetIDs ...flow.Identifier) (*message.Message, error) {
	// encode the payload using the configured codec
	payload, err := n.codec.Encode(event)
	if err != nil {
		return nil, fmt.Errorf("could not encode event: %w", err)
	}

	//bs := binstat.EnterTimeVal(binstat.BinNet+":wire<3payload2message", int64(len(payload)))
	//defer binstat.Leave(bs)

	eventId, err := EventId(channel, payload)
	if err != nil {
		return nil, fmt.Errorf("could not generate event id for message: %x", err)
	}

	var emTargets [][]byte
	for _, targetID := range targetIDs {
		tempID := targetID // avoid capturing loop variable
		emTargets = append(emTargets, tempID[:])
	}

	// get origin ID
	selfID := n.me.NodeID()
	originID := selfID[:]

	// get message type from event type and remove the asterisk prefix if present
	msgType := MessageType(event)

	// cast event to a libp2p.Message
	msg := &message.Message{
		ChannelID: channel.String(),
		TargetIDs: emTargets,
		Payload:   payload,

		// TODO: these fields should be derived by the receiver and removed from the message
		EventID:  eventId,
		OriginID: originID,
		Type:     msgType,
	}

	return msg, nil
}

// UnicastOnChannel sends the message in a reliable way to the given recipient.
// It uses 1-1 direct messaging over the underlying network to deliver the message.
// It returns an error if unicasting fails.
func (n *Network) UnicastOnChannel(channel channels.Channel, message interface{}, targetID flow.Identifier) error {
	if targetID == n.me.NodeID() {
		n.logger.Debug().Msg("network skips self unicasting")
		return nil
	}

	// generates network message (encoding) based on list of recipients
	msg, err := n.genNetworkMessage(channel, message, targetID)
	if err != nil {
		return fmt.Errorf("unicast could not generate network message: %w", err)
	}

	err = n.mw.SendDirect(msg, targetID)
	if err != nil {
		return fmt.Errorf("failed to send message to %x: %w", targetID, err)
	}

	return nil
}

// PublishOnChannel sends the message in an unreliable way to the given recipients.
// In this context, unreliable means that the message is published over a libp2p pub-sub
// channel and can be read by any node subscribed to that channel.
// The selector could be used to optimize or restrict delivery.
func (n *Network) PublishOnChannel(channel channels.Channel, message interface{}, targetIDs ...flow.Identifier) error {
	filteredIDs := flow.IdentifierList(targetIDs).Filter(n.removeSelfFilter())

	if len(filteredIDs) == 0 {
		return network.EmptyTargetList
	}

	err := n.sendOnChannel(channel, message, filteredIDs)

	if err != nil {
		return fmt.Errorf("failed to publish on channel %s: %w", channel, err)
	}

	return nil
}

// MulticastOnChannel unreliably sends the specified event over the channel to randomly selected 'num' number of recipients
// selected from the specified targetIDs.
func (n *Network) MulticastOnChannel(channel channels.Channel, message interface{}, num uint, targetIDs ...flow.Identifier) error {
	selectedIDs := flow.IdentifierList(targetIDs).Filter(n.removeSelfFilter()).Sample(num)

	if len(selectedIDs) == 0 {
		return network.EmptyTargetList
	}

	err := n.sendOnChannel(channel, message, selectedIDs)

	// publishes the message to the selected targets
	if err != nil {
		return fmt.Errorf("failed to multicast on channel %s: %w", channel, err)
	}

	return nil
}

// removeSelfFilter removes the flow.Identifier of this node if present, from the list of nodes
func (n *Network) removeSelfFilter() flow.IdentifierFilter {
	return func(id flow.Identifier) bool {
		return id != n.me.NodeID()
	}
}

// sendOnChannel sends the message on channel to targets.
func (n *Network) sendOnChannel(channel channels.Channel, message interface{}, targetIDs []flow.Identifier) error {
	n.logger.Debug().
		Interface("message", message).
		Str("channel", channel.String()).
		Str("target_ids", fmt.Sprintf("%v", targetIDs)).
		Msg("sending new message on channel")

	// generate network message (encoding) based on list of recipients
	msg, err := n.genNetworkMessage(channel, message, targetIDs...)
	if err != nil {
		return fmt.Errorf("failed to generate network message for channel %s: %w", channel, err)
	}

	// publish the message through the channel, however, the message
	// is only restricted to targetIDs (if they subscribed to channel).
	err = n.mw.Publish(msg, channel)
	if err != nil {
		return fmt.Errorf("failed to send message on channel %s: %w", channel, err)
	}

	return nil
}

// queueSubmitFunc submits the message to the engine synchronously. It is the callback for the queue worker
// when it gets a message from the queue
func (n *Network) queueSubmitFunc(message interface{}) {
	qm := message.(queue.QMessage)

	lg := n.logger.With().
		Str("channel_id", qm.Target.String()).
		Str("sender_id", qm.SenderID.String()).
		Logger()

	eng, err := n.subscriptionManager.GetEngine(qm.Target)
	if err != nil {
		// This means the message was received on a channel that the node has not registered an engine for
		lg.Err(err).
			Bool(logging.KeySuspicious, true).
			Msg("failed to submit message")
		return
	}

	lg.Debug().Msg("submitting message to engine")

	n.metrics.MessageProcessingStarted(qm.Target.String())

	// submits the message to the engine synchronously and
	// tracks its processing time.
	startTimestamp := time.Now()

	err = eng.Process(qm.Target, qm.SenderID, qm.Payload)
	if err != nil {
		lg.Err(err).Msg("engine failed to process message")
	}
	lg.Debug().Msg("message processed by engine")

	n.metrics.MessageProcessingFinished(qm.Target.String(), time.Since(startTimestamp))
}

func (n *Network) Topology() flow.IdentityList {
	return n.topology.Fanout(n.Identities())
}

func EventId(channel channels.Channel, payload []byte) (hash.Hash, error) {
	// use a hash with an engine-specific salt to get the payload hash
	h := hash.NewSHA3_384()
	_, err := h.Write([]byte(eventIDPackingPrefix + channel))
	if err != nil {
		return nil, fmt.Errorf("could not hash channel as salt: %w", err)
	}

	_, err = h.Write(payload)
	if err != nil {
		return nil, fmt.Errorf("could not hash event: %w", err)
	}

	return h.SumHash(), nil
}

func MessageType(decodedPayload interface{}) string {
	return strings.TrimLeft(fmt.Sprintf("%T", decodedPayload), "*")
}
