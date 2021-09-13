// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package p2p

import (
	"bufio"
	"context"
	"fmt"
	"sync"
	"time"

	ggio "github.com/gogo/protobuf/io"
	libp2pnetwork "github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/peerstore"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/id"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/network/message"
	"github.com/onflow/flow-go/network/validator"
	psValidator "github.com/onflow/flow-go/network/validator/pubsub"
	_ "github.com/onflow/flow-go/utils/binstat"
)

type communicationMode int

const (
	NoOp communicationMode = iota
	OneToOne
	OneToK
)

const (
	_  = iota
	kb = 1 << (10 * iota)
	mb
	gb
)

const (

	// defines maximum message size in publish and multicast modes
	DefaultMaxPubSubMsgSize = 5 * mb // 5 mb

	// defines maximum message size in unicast mode for most messages
	DefaultMaxUnicastMsgSize = 10 * mb // 10 mb

	// defines maximum message size in unicast mode for large messages
	LargeMsgMaxUnicastMsgSize = gb // 1 gb

	// default maximum time to wait for a default unicast request to complete
	// assuming at least a 1mb/sec connection
	DefaultUnicastTimeout = 5 * time.Second

	// maximum time to wait for a unicast request to complete for large message size
	LargeMsgUnicastTimeout = 1000 * time.Second
)

// Middleware handles the input & output on the direct connections we have to
// our neighbours on the peer-to-peer network.
type Middleware struct {
	sync.Mutex
	ctx                        context.Context
	cancel                     context.CancelFunc
	log                        zerolog.Logger
	ov                         network.Overlay
	wg                         *sync.WaitGroup
	libP2PNode                 *Node
	libP2PNodeFactory          LibP2PFactoryFunc
	me                         flow.Identifier
	metrics                    module.NetworkMetrics
	rootBlockID                string
	validators                 []network.MessageValidator
	peerManagerFactory         PeerManagerFactoryFunc
	peerManager                *PeerManager
	unicastMessageTimeout      time.Duration
	connectionGating           bool
	idTranslator               IDTranslator
	idProvider                 id.IdentifierProvider
	previousProtocolStatePeers []peer.AddrInfo
}

type MiddlewareOption func(*Middleware)

func WithIdentifierProvider(provider id.IdentifierProvider) MiddlewareOption {
	return func(mw *Middleware) {
		mw.idProvider = provider
	}
}

func WithMessageValidators(validators ...network.MessageValidator) MiddlewareOption {
	return func(mw *Middleware) {
		mw.validators = validators
	}
}

func WithPeerManager(peerManagerFunc PeerManagerFactoryFunc) MiddlewareOption {
	return func(mw *Middleware) {
		mw.peerManagerFactory = peerManagerFunc
	}
}

// NewMiddleware creates a new middleware instance
// libP2PNodeFactory is the factory used to create a LibP2PNode
// flowID is this node's Flow ID
// metrics is the interface to report network related metrics
// peerUpdateInterval is the interval when the PeerManager's peer update runs
// unicastMessageTimeout is the timeout used for unicast messages
// connectionGating if set to True, restricts this node to only talk to other nodes which are part of the identity list
// managePeerConnections if set to True, enables the default PeerManager which continuously updates the node's peer connections
// validators are the set of the different message validators that each inbound messages is passed through
func NewMiddleware(
	log zerolog.Logger,
	libP2PNodeFactory LibP2PFactoryFunc,
	flowID flow.Identifier,
	metrics module.NetworkMetrics,
	rootBlockID string,
	unicastMessageTimeout time.Duration,
	connectionGating bool,
	idTranslator IDTranslator,
	opts ...MiddlewareOption,
) *Middleware {
	ctx, cancel := context.WithCancel(context.Background())

	if unicastMessageTimeout <= 0 {
		unicastMessageTimeout = DefaultUnicastTimeout
	}

	// create the node entity and inject dependencies & config
	mw := &Middleware{
		ctx:                   ctx,
		cancel:                cancel,
		log:                   log,
		wg:                    &sync.WaitGroup{},
		me:                    flowID,
		libP2PNodeFactory:     libP2PNodeFactory,
		metrics:               metrics,
		rootBlockID:           rootBlockID,
		validators:            DefaultValidators(log, flowID),
		unicastMessageTimeout: unicastMessageTimeout,
		connectionGating:      connectionGating,
		peerManagerFactory:    nil,
		idTranslator:          idTranslator,
	}

	for _, opt := range opts {
		opt(mw)
	}

	return mw
}

func DefaultValidators(log zerolog.Logger, flowID flow.Identifier) []network.MessageValidator {
	return []network.MessageValidator{
		validator.ValidateNotSender(flowID),   // validator to filter out messages sent by this node itself
		validator.ValidateTarget(log, flowID), // validator to filter out messages not intended for this node
	}
}

func (m *Middleware) topologyPeers() (peer.IDSlice, error) {
	identities, err := m.ov.Topology()
	if err != nil {
		return nil, err
	}

	return m.peerIDs(identities.NodeIDs()), nil
}

func (m *Middleware) allPeers() peer.IDSlice {
	return m.peerIDs(m.idProvider.Identifiers())
}

func (m *Middleware) peerIDs(flowIDs flow.IdentifierList) peer.IDSlice {
	result := make([]peer.ID, len(flowIDs))

	for _, fid := range flowIDs {
		pid, err := m.idTranslator.GetPeerID(fid)
		if err != nil {
			// We probably don't need to fail the entire function here, since the other
			// translations may still succeed
			m.log.Err(err).Str("flowID", fid.String()).Msg("failed to translate to peer ID")
			continue
		}

		result = append(result, pid)
	}

	return result
}

// Me returns the flow identifier of the this middleware
func (m *Middleware) Me() flow.Identifier {
	return m.me
}

// GetIPPort returns the ip address and port number associated with the middleware
func (m *Middleware) GetIPPort() (string, string, error) {
	return m.libP2PNode.GetIPPort()
}

func (m *Middleware) UpdateNodeAddresses() {
	m.log.Info().Msg("Updating protocol state node addresses")

	ids := m.ov.Identities()
	newInfos, invalid := peerInfosFromIDs(ids)

	for id, err := range invalid {
		m.log.Err(err).Str("node_id", id.String()).Msg("failed to extract peer info from identity")
	}

	m.Lock()
	defer m.Unlock()

	// set old addresses to expire
	for _, oldInfo := range m.previousProtocolStatePeers {
		m.libP2PNode.host.Peerstore().SetAddrs(oldInfo.ID, oldInfo.Addrs, peerstore.TempAddrTTL)
	}

	for _, info := range newInfos {
		m.libP2PNode.host.Peerstore().SetAddrs(info.ID, info.Addrs, peerstore.PermanentAddrTTL)
	}

	m.previousProtocolStatePeers = newInfos
}

// Start will start the middleware.
func (m *Middleware) Start(ov network.Overlay) error {
	m.ov = ov
	libP2PNode, err := m.libP2PNodeFactory()

	if m.idProvider == nil {
		m.idProvider = NewPeerstoreIdentifierProvider(m.log, libP2PNode.host, m.idTranslator)
	}

	if err != nil {
		return fmt.Errorf("could not create libp2p node: %w", err)
	}
	m.libP2PNode = libP2PNode
	m.libP2PNode.SetFlowProtocolStreamHandler(m.handleIncomingStream)

	m.UpdateNodeAddresses()

	if m.connectionGating {
		m.libP2PNode.UpdateAllowList(m.allPeers())
	}

	// create and use a peer manager if a peer manager factory was passed in during initialization
	if m.peerManagerFactory != nil {

		m.peerManager, err = m.peerManagerFactory(m.libP2PNode.host, m.topologyPeers, m.log)
		if err != nil {
			return fmt.Errorf("failed to create peer manager: %w", err)
		}

		select {
		case <-m.peerManager.Ready():
			m.log.Debug().Msg("peer manager successfully started")
		case <-time.After(30 * time.Second):
			return fmt.Errorf("could not start peer manager")
		}
	}

	return nil
}

// Stop will end the execution of the middleware and wait for it to end.
func (m *Middleware) Stop() {
	mgr, found := m.peerMgr()
	if found {
		// stops peer manager
		<-mgr.Done()
		m.log.Debug().Msg("peer manager successfully stopped")
	}

	// stops libp2p
	done, err := m.libP2PNode.Stop()
	if err != nil {
		m.log.Error().Err(err).Msg("could not stop libp2p node")
	} else {
		<-done
		m.log.Debug().Msg("libp2p node successfully stopped")
	}

	// cancel the context (this also signals any lingering libp2p go routines to exit)
	m.cancel()

	// wait for the readConnection and readSubscription routines to stop
	m.wg.Wait()
}

// SendDirect sends msg on a 1-1 direct connection to the target ID. It models a guaranteed delivery asynchronous
// direct one-to-one connection on the underlying network. No intermediate node on the overlay is utilized
// as the router.
//
// Dispatch should be used whenever guaranteed delivery to a specific target is required. Otherwise, Publish is
// a more efficient candidate.
func (m *Middleware) SendDirect(msg *message.Message, targetID flow.Identifier) error {
	// translates identifier to peer id
	peerID, err := m.idTranslator.GetPeerID(targetID)
	if err != nil {
		return fmt.Errorf("could not find peer id for target id: %w", err)
	}

	maxMsgSize := unicastMaxMsgSize(msg)
	if msg.Size() > maxMsgSize {
		// message size goes beyond maximum size that the serializer can handle.
		// proceeding with this message results in closing the connection by the target side, and
		// delivery failure.
		return fmt.Errorf("message size %d exceeds configured max message size %d", msg.Size(), maxMsgSize)
	}

	maxTimeout := m.unicastMaxMsgDuration(msg)
	// pass in a context with timeout to make the unicast call fail fast
	ctx, cancel := context.WithTimeout(m.ctx, maxTimeout)
	defer cancel()

	// create new stream
	// (streams don't need to be reused and are fairly inexpensive to be created for each send.
	// A stream creation does NOT incur an RTT as stream negotiation happens as part of the first message
	// sent out the the receiver
	stream, err := m.libP2PNode.CreateStream(ctx, peerID)
	if err != nil {
		return fmt.Errorf("failed to create stream for %s :%w", targetID, err)
	}

	// create a gogo protobuf writer
	bufw := bufio.NewWriter(stream)
	writer := ggio.NewDelimitedWriter(bufw)

	err = writer.WriteMsg(msg)
	if err != nil {
		return fmt.Errorf("failed to send message to %s: %w", targetID, err)
	}

	// flush the stream
	err = bufw.Flush()
	if err != nil {
		return fmt.Errorf("failed to flush stream for %s: %w", targetID, err)
	}

	// close the stream immediately
	err = stream.Close()
	if err != nil {
		return fmt.Errorf("failed to close the stream for %s: %w", targetID, err)
	}

	channel := metrics.ChannelOneToOne
	if _, isStaked := m.ov.Identities().ByNodeID(targetID); !isStaked {
		channel = metrics.ChannelOneToOneUnstaked
	}
	// OneToOne communication metrics are reported with topic OneToOne
	m.metrics.NetworkMessageSent(msg.Size(), channel, msg.Type)

	return nil
}

// handleIncomingStream handles an incoming stream from a remote peer
// it is a callback that gets called for each incoming stream by libp2p with a new stream object
func (m *Middleware) handleIncomingStream(s libp2pnetwork.Stream) {

	// qualify the logger with local and remote address
	log := streamLogger(m.log, s)

	log.Info().Msg("incoming stream received")

	nodeID, err := m.idTranslator.GetFlowID(s.Conn().RemotePeer())
	if err != nil {
		log.Err(err).Str("peer_id", s.Conn().RemotePeer().Pretty()).Msg("could not translate peer ID of incoming stream")
	}
	_, isStaked := m.ov.Identities().ByNodeID(nodeID)

	//create a new readConnection with the context of the middleware
	conn := newReadConnection(m.ctx, s, m.processAuthenticatedMessage, log, m.metrics, LargeMsgMaxUnicastMsgSize, isStaked)

	// kick off the receive loop to continuously receive messages
	m.wg.Add(1)
	go conn.receiveLoop(m.wg)
}

// Subscribe subscribes the middleware to a channel.
func (m *Middleware) Subscribe(channel network.Channel) error {

	topic := engine.TopicFromChannel(channel, m.rootBlockID)

	var validators []psValidator.MessageValidator
	if !engine.UnstakedChannels().Contains(channel) {
		// for channels used by the staked nodes, add the topic validator to filter out messages from non-staked nodes
		validators = append(validators, psValidator.StakedValidator(m.ov.Identity))
	}

	s, err := m.libP2PNode.Subscribe(m.ctx, topic, validators...)
	if err != nil {
		return fmt.Errorf("failed to subscribe for channel %s: %w", channel, err)
	}

	// create a new readSubscription with the context of the middleware
	rs := newReadSubscription(m.ctx, s, m.processAuthenticatedMessage, m.log, m.metrics)
	m.wg.Add(1)

	// kick off the receive loop to continuously receive messages
	go rs.receiveLoop(m.wg)

	// update peers to add some nodes interested in the same topic as direct peers
	m.peerManagerUpdate()

	return nil
}

// Unsubscribe unsubscribes the middleware from a channel.
func (m *Middleware) Unsubscribe(channel network.Channel) error {
	topic := engine.TopicFromChannel(channel, m.rootBlockID)
	err := m.libP2PNode.UnSubscribe(topic)
	if err != nil {
		return fmt.Errorf("failed to unsubscribe from channel %s: %w", channel, err)
	}

	// update peers to remove nodes subscribed to channel
	m.peerManagerUpdate()

	return nil
}

// processAuthenticatedMessage processes a message and a source (indicated by its peer ID) and eventually passes it to the overlay
// In particular, it populates the `OriginID` field of the message with a Flow ID.
// The assumption is that the message has been authenticated at the network level (libp2p) to originate from the peer with ID `peerID`
// this requirement is fulfilled by e.g. the output of readConnection and readSubscription
func (m *Middleware) processAuthenticatedMessage(msg *message.Message, peerID peer.ID) {
	flowID, err := m.idTranslator.GetFlowID(peerID)
	if err != nil {
		m.log.Warn().Err(err).Msgf("received message from unknown peer %v, and was dropped", peerID.String())
		return
	}

	msg.OriginID = flowID[:]

	m.processMessage(msg)
}

// processMessage processes a message and eventually passes it to the overlay
func (m *Middleware) processMessage(msg *message.Message) {

	// run through all the message validators
	for _, v := range m.validators {
		// if any one fails, stop message propagation
		if !v.Validate(*msg) {
			return
		}
	}

	// if validation passed, send the message to the overlay
	err := m.ov.Receive(flow.HashToID(msg.OriginID), msg)
	if err != nil {
		m.log.Error().Err(err).Msg("could not deliver payload")
	}
}

// Publish publishes a message on the channel. It models a distributed broadcast where the message is meant for all or
// a many nodes subscribing to the channel. It does not guarantee the delivery though, and operates on a best
// effort.
func (m *Middleware) Publish(msg *message.Message, channel network.Channel) error {

	// convert the message to bytes to be put on the wire.
	//bs := binstat.EnterTime(binstat.BinNet + ":wire<4message2protobuf")
	data, err := msg.Marshal()
	//binstat.LeaveVal(bs, int64(len(data)))
	if err != nil {
		return fmt.Errorf("failed to marshal the message: %w", err)
	}

	msgSize := len(data)
	if msgSize > DefaultMaxPubSubMsgSize {
		// libp2p pubsub will silently drop the message if its size is greater than the configured pubsub max message size
		// hence return an error as this message is undeliverable
		return fmt.Errorf("message size %d exceeds configured max message size %d", msgSize, DefaultMaxPubSubMsgSize)
	}

	topic := engine.TopicFromChannel(channel, m.rootBlockID)

	// publish the bytes on the topic
	err = m.libP2PNode.Publish(m.ctx, topic, data)
	if err != nil {
		return fmt.Errorf("failed to publish the message: %w", err)
	}

	m.metrics.NetworkMessageSent(len(data), string(channel), msg.Type)

	return nil
}

// Ping pings the target node and returns the ping RTT or an error
func (m *Middleware) Ping(targetID flow.Identifier) (message.PingResponse, time.Duration, error) {
	peerID, err := m.idTranslator.GetPeerID(targetID)
	if err != nil {
		return message.PingResponse{}, -1, fmt.Errorf("could not find peer id for target id: %w", err)
	}

	return m.libP2PNode.Ping(m.ctx, peerID)
}

// UpdateAllowList fetches the most recent identifiers of the nodes from overlay
// and updates the underlying libp2p node.
func (m *Middleware) UpdateAllowList() {
	// update libp2pNode's approve lists if this middleware also does connection gating
	if m.connectionGating {
		m.libP2PNode.UpdateAllowList(m.allPeers())
	}

	// update peer connections if this middleware also does peer management
	m.peerManagerUpdate()
}

func (m *Middleware) IdentifierProvider() id.IdentifierProvider {
	return m.idProvider
}

// IsConnected returns true if this node is connected to the node with id nodeID.
func (m *Middleware) IsConnected(nodeID flow.Identifier) (bool, error) {
	peerID, err := m.idTranslator.GetPeerID(nodeID)
	if err != nil {
		return false, fmt.Errorf("could not find peer id for target id: %w", err)
	}
	return m.libP2PNode.IsConnected(peerID)
}

// unicastMaxMsgSize returns the max permissible size for a unicast message
func unicastMaxMsgSize(msg *message.Message) int {
	switch msg.Type {
	case "messages.ChunkDataResponse":
		return LargeMsgMaxUnicastMsgSize
	default:
		return DefaultMaxUnicastMsgSize
	}
}

// unicastMaxMsgDuration returns the max duration to allow for a unicast send to complete
func (m *Middleware) unicastMaxMsgDuration(msg *message.Message) time.Duration {
	switch msg.Type {
	case "messages.ChunkDataResponse":
		if LargeMsgUnicastTimeout > m.unicastMessageTimeout {
			return LargeMsgMaxUnicastMsgSize
		}
		return m.unicastMessageTimeout
	default:
		return m.unicastMessageTimeout
	}
}

// peerManagerUpdate request an update from the peer manager to connect to new peers and disconnect from unwanted peers
func (m *Middleware) peerManagerUpdate() {
	mgr, found := m.peerMgr()
	if found {
		mgr.RequestPeerUpdate()
	}
}

// peerMgr returns the PeerManager and true if this middleware was started with one, (nil, false) otherwise
func (m *Middleware) peerMgr() (*PeerManager, bool) {
	if m.peerManager != nil {
		return m.peerManager, true
	}
	return nil, false
}
