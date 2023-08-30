package p2p

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"sync"
	"time"

	ggio "github.com/gogo/protobuf/io"
	"github.com/ipfs/go-datastore"
	libp2pnetwork "github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/peerstore"
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
	"github.com/onflow/flow-go/network/codec"
	"github.com/onflow/flow-go/network/internal/p2putils"
	"github.com/onflow/flow-go/network/message"
	"github.com/onflow/flow-go/network/p2p/blob"
	"github.com/onflow/flow-go/network/p2p/ping"
	"github.com/onflow/flow-go/network/p2p/utils"
	"github.com/onflow/flow-go/network/queue"
	"github.com/onflow/flow-go/network/slashing"
	"github.com/onflow/flow-go/network/validator"
	flowpubsub "github.com/onflow/flow-go/network/validator/pubsub"
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
	sporkId                     flow.Identifier
	identityProvider            module.IdentityProvider
	identityTranslator          IDTranslator
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

	bitswapMetrics             module.BitswapMetrics
	libp2pNode                 LibP2PNode
	previousProtocolStatePeers []peer.AddrInfo
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
	SporkId             flow.Identifier
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
		sporkId:                     param.SporkId,
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

func (n *Network) Receive(msg network.IncomingMessageScope) error {
	n.metrics.InboundMessageReceived(msg.Size(), msg.Channel().String(), msg.Protocol().String(), msg.PayloadType())

	err := n.processNetworkMessage(msg)
	if err != nil {
		return fmt.Errorf("could not process message: %w", err)
	}
	return nil
}

func (n *Network) processNetworkMessage(msg network.IncomingMessageScope) error {
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

	msg, err := message.NewOutgoingScope(
		flow.IdentifierList{targetID},
		channels.TopicFromChannel(channel, n.sporkId),
		payload,
		n.codec.Encode,
		message.ProtocolTypeUnicast)
	if err != nil {
		return fmt.Errorf("could not generate outgoing message scope for unicast: %w", err)
	}

	n.metrics.UnicastMessageSendingStarted(channel.String())
	defer n.metrics.UnicastMessageSendingCompleted(channel.String())
	err = n.mw.SendDirect(msg)
	if err != nil {
		return fmt.Errorf("failed to send message to %x: %w", targetID, err)
	}

	n.metrics.OutboundMessageSent(msg.Size(), channel.String(), message.ProtocolTypeUnicast.String(), msg.PayloadType())

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
	scope, err := message.NewOutgoingScope(
		targetIDs,
		channels.TopicFromChannel(channel, n.sporkId),
		msg,
		n.codec.Encode,
		message.ProtocolTypePubSub)
	if err != nil {
		return fmt.Errorf("failed to generate outgoing message scope %s: %w", channel, err)
	}

	// publish the message through the channel, however, the message
	// is only restricted to targetIDs (if they subscribed to channel).
	err = n.mw.Publish(scope)
	if err != nil {
		return fmt.Errorf("failed to send message on channel %s: %w", channel, err)
	}

	n.metrics.OutboundMessageSent(scope.Size(), channel.String(), message.ProtocolTypePubSub.String(), scope.PayloadType())

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

func (n *Network) NewBlobService(channel channels.Channel, ds datastore.Batching, opts ...network.BlobServiceOption) network.BlobService {
	return blob.NewBlobService(n.libp2pNode.Host(), n.libp2pNode.Routing(), channel.String(), ds, n.bitswapMetrics, n.logger, opts...)
}

func (n *Network) NewPingService(pingProtocol protocol.ID, provider network.PingInfoProvider) network.PingService {
	return ping.NewPingService(n.libp2pNode.Host(), pingProtocol, n.logger, provider)
}

func (n *Network) peerIDs(flowIDs flow.IdentifierList) peer.IDSlice {
	result := make([]peer.ID, 0, len(flowIDs))

	for _, fid := range flowIDs {
		pid, err := n.identityTranslator.GetPeerID(fid)
		if err != nil {
			// We probably don't need to fail the entire function here, since the other
			// translations may still succeed
			n.logger.Err(err).Hex("node_id", logging.ID(fid)).Msg("failed to translate to peer ID")
			continue
		}

		result = append(result, pid)
	}

	return result
}

func (n *Network) UpdateNodeAddresses() {
	n.logger.Info().Msg("Updating protocol state node addresses")

	ids := n.Identities()
	newInfos, invalid := utils.PeerInfosFromIDs(ids)

	for id, err := range invalid {
		n.logger.
			Err(err).
			Hex("node_id", logging.ID(id)).
			Msg("failed to extract peer info from identity")
	}

	n.Lock()
	defer n.Unlock()

	// set old addresses to expire
	for _, oldInfo := range n.previousProtocolStatePeers {
		n.libp2pNode.Host().Peerstore().SetAddrs(oldInfo.ID, oldInfo.Addrs, peerstore.TempAddrTTL)
	}

	for _, info := range newInfos {
		n.libp2pNode.Host().Peerstore().SetAddrs(info.ID, info.Addrs, peerstore.PermanentAddrTTL)
	}

	n.previousProtocolStatePeers = newInfos
}

func (m *Middleware) SetOverlay(ov network.Overlay) {
	m.ov = ov
}

// SetSlashingViolationsConsumer sets the slashing violations consumer.
func (m *Middleware) SetSlashingViolationsConsumer(consumer network.ViolationsConsumer) {
	m.slashingViolationsConsumer = consumer
}

// authorizedPeers is a peer manager callback used by the underlying libp2p node that updates who can connect to this node (as
// well as who this node can connect to).
// and who is not allowed to connect to this node. This function is called by the peer manager and connection gater components
// of libp2p.
//
// Args:
// none
// Returns:
// - peer.IDSlice: a list of peer IDs that are allowed to connect to this node (and that this node can connect to). Any peer
// not in this list is assumed to be disconnected from this node (if connected) and not allowed to connect to this node.
// This is the guarantee that the underlying libp2p node implementation makes.
func (m *Middleware) authorizedPeers() peer.IDSlice {
	peerIDs := make([]peer.ID, 0)
	for _, id := range m.peerIDs(m.ov.Topology().NodeIDs()) {
		peerAllowed := true
		for _, filter := range m.peerManagerFilters {
			if err := filter(id); err != nil {
				m.log.Debug().
					Err(err).
					Str("peer_id", id.String()).
					Msg("filtering topology peer")

				peerAllowed = false
				break
			}
		}

		if peerAllowed {
			peerIDs = append(peerIDs, id)
		}
	}

	return peerIDs
}

func (m *Middleware) OnDisallowListNotification(notification *network.DisallowListingUpdate) {
	for _, pid := range m.peerIDs(notification.FlowIds) {
		m.libP2PNode.OnDisallowListNotification(pid, notification.Cause)
	}
}

func (m *Middleware) OnAllowListNotification(notification *network.AllowListingUpdate) {
	for _, pid := range m.peerIDs(notification.FlowIds) {
		m.libP2PNode.OnAllowListNotification(pid, notification.Cause)
	}
}

// SendDirect sends msg on a 1-1 direct connection to the target ID. It models a guaranteed delivery asynchronous
// direct one-to-one connection on the underlying network. No intermediate node on the overlay is utilized
// as the router.
//
// Dispatch should be used whenever guaranteed delivery to a specific target is required. Otherwise, Publish is
// a more efficient candidate.
//
// The following benign errors can be returned:
// - the peer ID for the target node ID cannot be found.
// - the msg size was too large.
// - failed to send message to peer.
//
// All errors returned from this function can be considered benign.
func (m *Middleware) SendDirect(msg network.OutgoingMessageScope) error {
	// since it is a unicast, we only need to get the first peer ID.
	peerID, err := m.idTranslator.GetPeerID(msg.TargetIds()[0])
	if err != nil {
		return fmt.Errorf("could not find peer id for target id: %w", err)
	}

	maxMsgSize := unicastMaxMsgSize(msg.PayloadType())
	if msg.Size() > maxMsgSize {
		// message size goes beyond maximum size that the serializer can handle.
		// proceeding with this message results in closing the connection by the target side, and
		// delivery failure.
		return fmt.Errorf("message size %d exceeds configured max message size %d", msg.Size(), maxMsgSize)
	}

	maxTimeout := m.unicastMaxMsgDuration(msg.PayloadType())

	// pass in a context with timeout to make the unicast call fail fast
	ctx, cancel := context.WithTimeout(m.ctx, maxTimeout)
	defer cancel()

	// protect the underlying connection from being inadvertently pruned by the peer manager while the stream and
	// connection creation is being attempted, and remove it from protected list once stream created.
	channel, ok := channels.ChannelFromTopic(msg.Topic())
	if !ok {
		return fmt.Errorf("could not find channel for topic %s", msg.Topic())
	}
	tag := fmt.Sprintf("%v:%v", channel, msg.PayloadType())
	m.libP2PNode.Host().ConnManager().Protect(peerID, tag)
	defer m.libP2PNode.Host().ConnManager().Unprotect(peerID, tag)

	// create new stream
	// streams don't need to be reused and are fairly inexpensive to be created for each send.
	// A stream creation does NOT incur an RTT as stream negotiation happens as part of the first message
	// sent out the receiver
	stream, err := m.libP2PNode.CreateStream(ctx, peerID)
	if err != nil {
		return fmt.Errorf("failed to create stream for %s: %w", msg.TargetIds()[0], err)
	}

	success := false

	defer func() {
		if success {
			// close the stream immediately
			err = stream.Close()
			if err != nil {
				err = fmt.Errorf("failed to close the stream for %s: %w", msg.TargetIds()[0], err)
			}
		} else {
			resetErr := stream.Reset()
			if resetErr != nil {
				m.log.Err(resetErr).Msg("failed to reset stream")
			}
		}
	}()

	deadline, _ := ctx.Deadline()
	err = stream.SetWriteDeadline(deadline)
	if err != nil {
		return fmt.Errorf("failed to set write deadline for stream: %w", err)
	}

	// create a gogo protobuf writer
	bufw := bufio.NewWriter(stream)
	writer := ggio.NewDelimitedWriter(bufw)

	err = writer.WriteMsg(msg.Proto())
	if err != nil {
		return fmt.Errorf("failed to send message to %s: %w", msg.TargetIds()[0], err)
	}

	// flush the stream
	err = bufw.Flush()
	if err != nil {
		return fmt.Errorf("failed to flush stream for %s: %w", msg.TargetIds()[0], err)
	}

	success = true

	return nil
}

// handleIncomingStream handles an incoming stream from a remote peer
// it is a callback that gets called for each incoming stream by libp2p with a new stream object
func (m *Middleware) handleIncomingStream(s libp2pnetwork.Stream) {
	// qualify the logger with local and remote address
	log := p2putils.StreamLogger(m.log, s)

	log.Info().Msg("incoming stream received")

	success := false

	remotePeer := s.Conn().RemotePeer()

	defer func() {
		if success {
			err := s.Close()
			if err != nil {
				log.Err(err).Msg("failed to close stream")
			}
		} else {
			err := s.Reset()
			if err != nil {
				log.Err(err).Msg("failed to reset stream")
			}
		}
	}()

	// check if peer is currently rate limited before continuing to process stream.
	if m.unicastRateLimiters.MessageRateLimiter.IsRateLimited(remotePeer) || m.unicastRateLimiters.BandWidthRateLimiter.IsRateLimited(remotePeer) {
		log.Debug().
			Bool(logging.KeySuspicious, true).
			Msg("dropping unicast stream from rate limited peer")
		return
	}

	// TODO: We need to allow per-topic timeouts and message size limits.
	// This allows us to configure higher limits for topics on which we expect
	// to receive large messages (e.g. Chunk Data Packs), and use the normal
	// limits for other topics. In order to enable this, we will need to register
	// a separate stream handler for each topic.
	ctx, cancel := context.WithTimeout(m.ctx, LargeMsgUnicastTimeout)
	defer cancel()

	deadline, _ := ctx.Deadline()

	err := s.SetReadDeadline(deadline)
	if err != nil {
		log.Err(err).Msg("failed to set read deadline for stream")
		return
	}

	// create the reader
	r := ggio.NewDelimitedReader(s, LargeMsgMaxUnicastMsgSize)
	for {
		if ctx.Err() != nil {
			return
		}

		// Note: message fields must not be trusted until explicitly validated
		var msg message.Message
		// read the next message (blocking call)
		err = r.ReadMsg(&msg)
		if err != nil {
			if err == io.EOF {
				break
			}

			m.log.Err(err).Msg("failed to read message")
			return
		}

		channel := channels.Channel(msg.ChannelID)
		topic := channels.TopicFromChannel(channel, m.sporkId)

		// ignore messages if node does not have subscription to topic
		if !m.libP2PNode.HasSubscription(topic) {
			violation := &network.Violation{
				Identity: nil, PeerID: remotePeer.String(), Channel: channel, Protocol: message.ProtocolTypeUnicast,
			}

			msgCode, err := codec.MessageCodeFromPayload(msg.Payload)
			if err != nil {
				violation.Err = err
				m.slashingViolationsConsumer.OnUnknownMsgTypeError(violation)
				return
			}

			// msg type is not guaranteed to be correct since it is set by the client
			_, what, err := codec.InterfaceFromMessageCode(msgCode)
			if err != nil {
				violation.Err = err
				m.slashingViolationsConsumer.OnUnknownMsgTypeError(violation)
				return
			}

			violation.MsgType = what
			violation.Err = ErrUnicastMsgWithoutSub
			m.slashingViolationsConsumer.OnUnauthorizedUnicastOnChannel(violation)
			return
		}

		// check if unicast messages have reached rate limit before processing next message
		if !m.unicastRateLimiters.MessageAllowed(remotePeer) {
			return
		}

		// check if we can get a role for logging and metrics label if this is not a public channel
		role := ""
		if !channels.IsPublicChannel(channels.Channel(msg.ChannelID)) {
			if identity, ok := m.ov.Identity(remotePeer); ok {
				role = identity.Role.String()
			}
		}

		// check unicast bandwidth rate limiter for peer
		if !m.unicastRateLimiters.BandwidthAllowed(
			remotePeer,
			role,
			msg.Size(),
			message.MessageType(msg.Payload),
			channels.Topic(msg.ChannelID)) {
			return
		}

		m.wg.Add(1)
		go func() {
			defer m.wg.Done()
			m.processUnicastStreamMessage(remotePeer, &msg)
		}()
	}

	success = true
}

// Subscribe subscribes the middleware to a channel.
// No errors are expected during normal operation.
func (m *Middleware) Subscribe(channel channels.Channel) error {
	topic := channels.TopicFromChannel(channel, m.sporkId)

	var peerFilter p2p.PeerFilter
	var validators []validator.PubSubMessageValidator
	if channels.IsPublicChannel(channel) {
		// NOTE: for public channels the callback used to check if a node is staked will
		// return true for every node.
		peerFilter = p2p.AllowAllPeerFilter()
	} else {
		// for channels used by the staked nodes, add the topic validator to filter out messages from non-staked nodes
		validators = append(validators, m.authorizedSenderValidator.PubSubMessageValidator(channel))

		// NOTE: For non-public channels the libP2P node topic validator will reject
		// messages from unstaked nodes.
		peerFilter = m.isProtocolParticipant()
	}

	topicValidator := flowpubsub.TopicValidator(m.log, peerFilter, validators...)
	s, err := m.libP2PNode.Subscribe(topic, topicValidator)
	if err != nil {
		return fmt.Errorf("could not subscribe to topic (%s): %w", topic, err)
	}

	// create a new readSubscription with the context of the middleware
	rs := newReadSubscription(s, m.processPubSubMessages, m.log)
	m.wg.Add(1)

	// kick off the receive loop to continuously receive messages
	go func() {
		defer m.wg.Done()
		rs.receiveLoop(m.ctx)
	}()

	// update peers to add some nodes interested in the same topic as direct peers
	m.libP2PNode.RequestPeerUpdate()

	return nil
}

// processPubSubMessages processes messages received from the pubsub subscription.
func (m *Middleware) processPubSubMessages(msg *message.Message, peerID peer.ID) {
	m.processAuthenticatedMessage(msg, peerID, message.ProtocolTypePubSub)
}

// Unsubscribe unsubscribes the middleware from a channel.
// The following benign errors are expected during normal operations from libP2P:
// - the libP2P node fails to unsubscribe to the topic created from the provided channel.
//
// All errors returned from this function can be considered benign.
func (m *Middleware) Unsubscribe(channel channels.Channel) error {
	topic := channels.TopicFromChannel(channel, m.sporkId)
	return m.libP2PNode.Unsubscribe(topic)
}

// processUnicastStreamMessage will decode, perform authorized sender validation and process a message
// sent via unicast stream. This func should be invoked in a separate goroutine to avoid creating a message decoding bottleneck.
func (m *Middleware) processUnicastStreamMessage(remotePeer peer.ID, msg *message.Message) {
	channel := channels.Channel(msg.ChannelID)

	// TODO: once we've implemented per topic message size limits per the TODO above,
	// we can remove this check
	maxSize, err := UnicastMaxMsgSizeByCode(msg.Payload)
	if err != nil {
		m.slashingViolationsConsumer.OnUnknownMsgTypeError(&network.Violation{
			Identity: nil, PeerID: remotePeer.String(), MsgType: "", Channel: channel, Protocol: message.ProtocolTypeUnicast, Err: err,
		})
		return
	}
	if msg.Size() > maxSize {
		// message size exceeded
		m.log.Error().
			Str("peer_id", remotePeer.String()).
			Str("channel", msg.ChannelID).
			Int("max_size", maxSize).
			Int("size", msg.Size()).
			Bool(logging.KeySuspicious, true).
			Msg("received message exceeded permissible message maxSize")
		return
	}

	// if message channel is not public perform authorized sender validation
	if !channels.IsPublicChannel(channel) {
		messageType, err := m.authorizedSenderValidator.Validate(remotePeer, msg.Payload, channel, message.ProtocolTypeUnicast)
		if err != nil {
			m.log.
				Error().
				Err(err).
				Str("peer_id", remotePeer.String()).
				Str("type", messageType).
				Str("channel", msg.ChannelID).
				Msg("unicast authorized sender validation failed")
			return
		}
	}
	m.processAuthenticatedMessage(msg, remotePeer, message.ProtocolTypeUnicast)
}

// processAuthenticatedMessage processes a message and a source (indicated by its peer ID) and eventually passes it to the overlay
// In particular, it populates the `OriginID` field of the message with a Flow ID translated from this source.
func (m *Middleware) processAuthenticatedMessage(msg *message.Message, peerID peer.ID, protocol message.ProtocolType) {
	originId, err := m.idTranslator.GetFlowID(peerID)
	if err != nil {
		// this error should never happen. by the time the message gets here, the peer should be
		// authenticated which means it must be known
		m.log.Error().
			Err(err).
			Str("peer_id", peerID.String()).
			Bool(logging.KeySuspicious, true).
			Msg("dropped message from unknown peer")
		return
	}

	channel := channels.Channel(msg.ChannelID)
	decodedMsgPayload, err := m.codec.Decode(msg.Payload)
	switch {
	case codec.IsErrUnknownMsgCode(err):
		// slash peer if message contains unknown message code byte
		violation := &network.Violation{
			PeerID: peerID.String(), OriginID: originId, Channel: channel, Protocol: protocol, Err: err,
		}
		m.slashingViolationsConsumer.OnUnknownMsgTypeError(violation)
		return
	case codec.IsErrMsgUnmarshal(err) || codec.IsErrInvalidEncoding(err):
		// slash if peer sent a message that could not be marshalled into the message type denoted by the message code byte
		violation := &network.Violation{
			PeerID: peerID.String(), OriginID: originId, Channel: channel, Protocol: protocol, Err: err,
		}
		m.slashingViolationsConsumer.OnInvalidMsgError(violation)
		return
	case err != nil:
		// this condition should never happen and indicates there's a bug
		// don't crash as a result of external inputs since that creates a DoS vector
		// collect slashing data because this could potentially lead to slashing
		err = fmt.Errorf("unexpected error during message validation: %w", err)
		violation := &network.Violation{
			PeerID: peerID.String(), OriginID: originId, Channel: channel, Protocol: protocol, Err: err,
		}
		m.slashingViolationsConsumer.OnUnexpectedError(violation)
		return
	}

	scope, err := message.NewIncomingScope(originId, protocol, msg, decodedMsgPayload)
	if err != nil {
		m.log.Error().
			Err(err).
			Str("peer_id", peerID.String()).
			Str("origin_id", originId.String()).
			Msg("could not create incoming message scope")
		return
	}

	m.processMessage(scope)
}

// processMessage processes a message and eventually passes it to the overlay
func (m *Middleware) processMessage(scope network.IncomingMessageScope) {
	logger := m.log.With().
		Str("channel", scope.Channel().String()).
		Str("type", scope.Protocol().String()).
		Int("msg_size", scope.Size()).
		Hex("origin_id", logging.ID(scope.OriginId())).
		Logger()

	// run through all the message validators
	for _, v := range m.validators {
		// if any one fails, stop message propagation
		if !v.Validate(scope) {
			logger.Debug().Msg("new message filtered by message validators")
			return
		}
	}

	logger.Debug().Msg("processing new message")

	// if validation passed, send the message to the overlay
	err := m.ov.Receive(scope)
	if err != nil {
		m.log.Error().Err(err).Msg("could not deliver payload")
	}
}

// Publish publishes a message on the channel. It models a distributed broadcast where the message is meant for all or
// a many nodes subscribing to the channel. It does not guarantee the delivery though, and operates on a best
// effort.
// The following benign errors are expected during normal operations:
// - the msg cannot be marshalled.
// - the msg size exceeds DefaultMaxPubSubMsgSize.
// - the libP2P node fails to publish the message.
//
// All errors returned from this function can be considered benign.
// TODO: DO NOT USE. Publish is ready to be removed from middleware. Use libp2pNode.Publish directly.
func (m *Middleware) Publish(msg network.OutgoingMessageScope) error {
	return m.libP2PNode.Publish(m.ctx, msg)
}

// unicastMaxMsgSize returns the max permissible size for a unicast message
func unicastMaxMsgSize(messageType string) int {
	switch messageType {
	case "*messages.ChunkDataResponse", "messages.ChunkDataResponse":
		return LargeMsgMaxUnicastMsgSize
	default:
		return DefaultMaxUnicastMsgSize
	}
}

// UnicastMaxMsgSizeByCode returns the max permissible size for a unicast message code
func UnicastMaxMsgSizeByCode(payload []byte) (int, error) {
	msgCode, err := codec.MessageCodeFromPayload(payload)
	if err != nil {
		return 0, err
	}
	_, messageType, err := codec.InterfaceFromMessageCode(msgCode)
	if err != nil {
		return 0, err
	}

	maxSize := unicastMaxMsgSize(messageType)
	return maxSize, nil
}

// unicastMaxMsgDuration returns the max duration to allow for a unicast send to complete
func (m *Middleware) unicastMaxMsgDuration(messageType string) time.Duration {
	switch messageType {
	case "*messages.ChunkDataResponse", "messages.ChunkDataResponse":
		if LargeMsgUnicastTimeout > m.unicastMessageTimeout {
			return LargeMsgUnicastTimeout
		}
		return m.unicastMessageTimeout
	default:
		return m.unicastMessageTimeout
	}
}
