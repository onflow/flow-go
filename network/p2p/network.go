package p2p

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/ipfs/go-datastore"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/network"
	alspmgr "github.com/onflow/flow-go/network/alsp/manager"
	netcache "github.com/onflow/flow-go/network/cache"
	"github.com/onflow/flow-go/network/channels"
	"github.com/onflow/flow-go/network/message"
	"github.com/onflow/flow-go/network/queue"
	"github.com/onflow/flow-go/network/slashing"
	_ "github.com/onflow/flow-go/utils/binstat"
	"github.com/onflow/flow-go/utils/logging"
)

// NotEjectedFilter is an identity filter that, when applied to the identity
// table at a given snapshot, returns all nodes that we should communicate with
// over the networking layer.
//
// NOTE: The protocol state includes nodes from the previous/next epoch that should
// be included in network communication. We omit any nodes that have been ejected.
var NotEjectedFilter = filter.Not(filter.Ejected)

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
	metrics                     module.NetworkCoreMetrics
	receiveCache                *netcache.ReceiveCache // used to deduplicate incoming messages
	queue                       network.MessageQueue
	subscriptionManager         network.SubscriptionManager // used to keep track of subscribed channels
	conduitFactory              network.ConduitFactory
	topology                    network.Topology
	registerEngineRequests      chan *registerEngineRequest
	registerBlobServiceRequests chan *registerBlobServiceRequest
	misbehaviorReportManager    network.MisbehaviorReportManager
	slashingViolationsConsumer  network.ViolationsConsumer
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

// NetworkConfig is a configuration struct for the network. It contains all the
// necessary components to create a new network.
type NetworkConfig struct {
	Logger              zerolog.Logger
	Codec               network.Codec
	Me                  module.Local
	MiddlewareFactory   func() (network.Middleware, error)
	Topology            network.Topology
	SubscriptionManager network.SubscriptionManager
	Metrics             module.NetworkCoreMetrics
	IdentityProvider    module.IdentityProvider
	ReceiveCache        *netcache.ReceiveCache
	ConduitFactory      network.ConduitFactory
	AlspCfg             *alspmgr.MisbehaviorReportManagerConfig
}

// NetworkConfigOption is a function that can be used to override network config parmeters.
type NetworkConfigOption func(*NetworkConfig)

// WithAlspConfig overrides the default misbehavior report manager config. It is mostly used for testing purposes.
// Note: do not override the default misbehavior report manager config in production unless you know what you are doing.
// Args:
// cfg: misbehavior report manager config
// Returns:
// NetworkConfigOption: network param option
func WithAlspConfig(cfg *alspmgr.MisbehaviorReportManagerConfig) NetworkConfigOption {
	return func(params *NetworkConfig) {
		params.AlspCfg = cfg
	}
}

// NetworkOption is a function that can be used to override network attributes.
// It is mostly used for testing purposes.
// Note: do not override network attributes in production unless you know what you are doing.
type NetworkOption func(*Network)

// WithAlspManager sets the misbehavior report manager for the network. It overrides the default
// misbehavior report manager that is created from the config.
// Note that this option is mostly used for testing purposes, do not use it in production unless you
// know what you are doing.
//
// Args:
//
//	mgr: misbehavior report manager
//
// Returns:
//
//	NetworkOption: network option
func WithAlspManager(mgr network.MisbehaviorReportManager) NetworkOption {
	return func(n *Network) {
		n.misbehaviorReportManager = mgr
	}
}

// NewNetwork creates a new naive overlay network, using the given middleware to
// communicate to direct peers, using the given codec for serialization, and
// using the given state & cache interfaces to track volatile information.
// csize determines the size of the cache dedicated to keep track of received messages
func NewNetwork(param *NetworkConfig, opts ...NetworkOption) (*Network, error) {
	mw, err := param.MiddlewareFactory()
	if err != nil {
		return nil, fmt.Errorf("could not create middleware: %w", err)
	}
	misbehaviorMngr, err := alspmgr.NewMisbehaviorReportManager(param.AlspCfg, mw)
	if err != nil {
		return nil, fmt.Errorf("could not create misbehavior report manager: %w", err)
	}

	n := &Network{
		logger:                      param.Logger.With().Str("component", "network").Logger(),
		codec:                       param.Codec,
		me:                          param.Me,
		mw:                          mw,
		receiveCache:                param.ReceiveCache,
		topology:                    param.Topology,
		metrics:                     param.Metrics,
		subscriptionManager:         param.SubscriptionManager,
		identityProvider:            param.IdentityProvider,
		conduitFactory:              param.ConduitFactory,
		registerEngineRequests:      make(chan *registerEngineRequest),
		registerBlobServiceRequests: make(chan *registerBlobServiceRequest),
		misbehaviorReportManager:    misbehaviorMngr,
	}

	for _, opt := range opts {
		opt(n)
	}

	n.slashingViolationsConsumer = slashing.NewSlashingViolationsConsumer(param.Logger, param.Metrics, n)
	n.mw.SetSlashingViolationsConsumer(n.slashingViolationsConsumer)
	n.mw.SetOverlay(n)

	if err := n.conduitFactory.RegisterAdapter(n); err != nil {
		return nil, fmt.Errorf("could not register network adapter: %w", err)
	}

	n.ComponentManager = component.NewComponentManagerBuilder().
		AddWorker(func(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
			n.logger.Debug().Msg("starting misbehavior manager")
			n.misbehaviorReportManager.Start(ctx)

			select {
			case <-n.misbehaviorReportManager.Ready():
				n.logger.Debug().Msg("misbehavior manager is ready")
				ready()
			case <-ctx.Done():
				// jumps to the end of the select statement to let a graceful shutdown.
			}

			<-ctx.Done()
			n.logger.Debug().Msg("stopping misbehavior manager")
			<-n.misbehaviorReportManager.Done()
			n.logger.Debug().Msg("misbehavior manager stopped")
		}).
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

func (n *Network) Receive(msg *network.IncomingMessageScope) error {
	n.metrics.InboundMessageReceived(msg.Size(), msg.Channel().String(), msg.Protocol().String(), msg.PayloadType())

	err := n.processNetworkMessage(msg)
	if err != nil {
		return fmt.Errorf("could not process message: %w", err)
	}
	return nil
}

func (n *Network) processNetworkMessage(msg *network.IncomingMessageScope) error {
	// checks the cache for deduplication and adds the message if not already present
	if !n.receiveCache.Add(msg.EventID()) {
		// drops duplicate message
		n.logger.Debug().
			Hex("sender_id", logging.ID(msg.OriginId())).
			Hex("event_id", msg.EventID()).
			Str("channel", msg.Channel().String()).
			Msg("dropping message due to duplication")

		n.metrics.DuplicateInboundMessagesDropped(msg.Channel().String(), msg.Protocol().String(), msg.PayloadType())

		return nil
	}

	// create queue message
	qm := queue.QMessage{
		Payload:  msg.DecodedPayload(),
		Size:     msg.Size(),
		Target:   msg.Channel(),
		SenderID: msg.OriginId(),
	}

	// insert the message in the queue
	err := n.queue.Insert(qm)
	if err != nil {
		return fmt.Errorf("failed to insert message in queue: %w", err)
	}

	return nil
}

// UnicastOnChannel sends the message in a reliable way to the given recipient.
// It uses 1-1 direct messaging over the underlying network to deliver the message.
// It returns an error if unicasting fails.
func (n *Network) UnicastOnChannel(channel channels.Channel, payload interface{}, targetID flow.Identifier) error {
	if targetID == n.me.NodeID() {
		n.logger.Debug().Msg("network skips self unicasting")
		return nil
	}

	msg, err := network.NewOutgoingScope(
		flow.IdentifierList{targetID},
		channel,
		payload,
		n.codec.Encode,
		message.ProtocolTypeUnicast)
	if err != nil {
		return fmt.Errorf("could not generate outgoing message scope for unicast: %w", err)
	}

	n.metrics.UnicastMessageSendingStarted(msg.Channel().String())
	defer n.metrics.UnicastMessageSendingCompleted(msg.Channel().String())
	err = n.mw.SendDirect(msg)
	if err != nil {
		return fmt.Errorf("failed to send message to %x: %w", targetID, err)
	}

	n.metrics.OutboundMessageSent(msg.Size(), msg.Channel().String(), message.ProtocolTypeUnicast.String(), msg.PayloadType())

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
	selectedIDs, err := flow.IdentifierList(targetIDs).Filter(n.removeSelfFilter()).Sample(num)
	if err != nil {
		return fmt.Errorf("sampling failed: %w", err)
	}

	if len(selectedIDs) == 0 {
		return network.EmptyTargetList
	}

	err = n.sendOnChannel(channel, message, selectedIDs)

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
func (n *Network) sendOnChannel(channel channels.Channel, msg interface{}, targetIDs []flow.Identifier) error {
	n.logger.Debug().
		Interface("message", msg).
		Str("channel", channel.String()).
		Str("target_ids", fmt.Sprintf("%v", targetIDs)).
		Msg("sending new message on channel")

	// generate network message (encoding) based on list of recipients
	scope, err := network.NewOutgoingScope(targetIDs, channel, msg, n.codec.Encode, message.ProtocolTypePubSub)
	if err != nil {
		return fmt.Errorf("failed to generate outgoing message scope %s: %w", channel, err)
	}

	// publish the message through the channel, however, the message
	// is only restricted to targetIDs (if they subscribed to channel).
	err = n.mw.Publish(scope)
	if err != nil {
		return fmt.Errorf("failed to send message on channel %s: %w", channel, err)
	}

	n.metrics.OutboundMessageSent(scope.Size(), scope.Channel().String(), message.ProtocolTypePubSub.String(), scope.PayloadType())

	return nil
}

// queueSubmitFunc submits the message to the engine synchronously. It is the callback for the queue worker
// when it gets a message from the queue
func (n *Network) queueSubmitFunc(message interface{}) {
	qm := message.(queue.QMessage)

	logger := n.logger.With().
		Str("channel_id", qm.Target.String()).
		Str("sender_id", qm.SenderID.String()).
		Logger()

	eng, err := n.subscriptionManager.GetEngine(qm.Target)
	if err != nil {
		// This means the message was received on a channel that the node has not registered an
		// engine for. This may be because the message was received during startup and the node
		// hasn't subscribed to the channel yet, or there is a bug.
		logger.Err(err).Msg("failed to submit message")
		return
	}

	logger.Debug().Msg("submitting message to engine")

	n.metrics.MessageProcessingStarted(qm.Target.String())

	// submits the message to the engine synchronously and
	// tracks its processing time.
	startTimestamp := time.Now()

	err = eng.Process(qm.Target, qm.SenderID, qm.Payload)
	if err != nil {
		logger.Err(err).Msg("failed to process message")
	}

	n.metrics.MessageProcessingFinished(qm.Target.String(), time.Since(startTimestamp))
}

func (n *Network) Topology() flow.IdentityList {
	return n.topology.Fanout(n.Identities())
}

// ReportMisbehaviorOnChannel reports the misbehavior of a node on sending a message to the current node that appears
// valid based on the networking layer but is considered invalid by the current node based on the Flow protocol.
// The misbehavior report is sent to the current node's networking layer on the given channel to be processed.
// Args:
// - channel: The channel on which the misbehavior report is sent.
// - report: The misbehavior report to be sent.
// Returns:
// none
func (n *Network) ReportMisbehaviorOnChannel(channel channels.Channel, report network.MisbehaviorReport) {
	n.misbehaviorReportManager.HandleMisbehaviorReport(channel, report)
}
