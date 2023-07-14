// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package middleware

import (
	"bufio"
	"context"
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
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/network/channels"
	"github.com/onflow/flow-go/network/codec"
	"github.com/onflow/flow-go/network/internal/p2putils"
	"github.com/onflow/flow-go/network/message"
	"github.com/onflow/flow-go/network/p2p"
	"github.com/onflow/flow-go/network/p2p/blob"
	"github.com/onflow/flow-go/network/p2p/p2pnode"
	"github.com/onflow/flow-go/network/p2p/ping"
	"github.com/onflow/flow-go/network/p2p/unicast/protocols"
	"github.com/onflow/flow-go/network/p2p/unicast/ratelimit"
	"github.com/onflow/flow-go/network/p2p/utils"
	"github.com/onflow/flow-go/network/validator"
	flowpubsub "github.com/onflow/flow-go/network/validator/pubsub"
	_ "github.com/onflow/flow-go/utils/binstat"
	"github.com/onflow/flow-go/utils/logging"
)

const (
	_ = iota
	_ = 1 << (10 * iota)
	mb
	gb
)

const (
	// DefaultMaxUnicastMsgSize defines maximum message size in unicast mode for most messages
	DefaultMaxUnicastMsgSize = 10 * mb // 10 mb

	// LargeMsgMaxUnicastMsgSize defines maximum message size in unicast mode for large messages
	LargeMsgMaxUnicastMsgSize = gb // 1 gb

	// DefaultUnicastTimeout is the default maximum time to wait for a default unicast request to complete
	// assuming at least a 1mb/sec connection
	DefaultUnicastTimeout = 5 * time.Second

	// LargeMsgUnicastTimeout is the maximum time to wait for a unicast request to complete for large message size
	LargeMsgUnicastTimeout = 1000 * time.Second
)

var (
	_ network.Middleware = (*Middleware)(nil)

	// ErrUnicastMsgWithoutSub error is provided to the slashing violations consumer in the case where
	// the middleware receives a message via unicast but does not have a corresponding subscription for
	// the channel in that message.
	ErrUnicastMsgWithoutSub = errors.New("middleware does not have subscription for the channel ID indicated in the unicast message received")
)

// Middleware handles the input & output on the direct connections we have to
// our neighbours on the peer-to-peer network.
type Middleware struct {
	sync.Mutex
	component.Component
	ctx context.Context
	log zerolog.Logger
	ov  network.Overlay

	// TODO: using a waitgroup here doesn't actually guarantee that we'll wait for all
	// goroutines to exit, because new goroutines could be started after we've already
	// returned from wg.Wait(). We need to solve this the right way using ComponentManager
	// and worker routines.
	wg                         sync.WaitGroup
	libP2PNode                 p2p.LibP2PNode
	preferredUnicasts          []protocols.ProtocolName
	bitswapMetrics             module.BitswapMetrics
	rootBlockID                flow.Identifier
	validators                 []network.MessageValidator
	peerManagerFilters         []p2p.PeerFilter
	unicastMessageTimeout      time.Duration
	idTranslator               p2p.IDTranslator
	previousProtocolStatePeers []peer.AddrInfo
	codec                      network.Codec
	slashingViolationsConsumer network.ViolationsConsumer
	unicastRateLimiters        *ratelimit.RateLimiters
	authorizedSenderValidator  *validator.AuthorizedSenderValidator
}

type OptionFn func(*Middleware)

func WithMessageValidators(validators ...network.MessageValidator) OptionFn {
	return func(mw *Middleware) {
		mw.validators = validators
	}
}

func WithPreferredUnicastProtocols(unicasts []protocols.ProtocolName) OptionFn {
	return func(mw *Middleware) {
		mw.preferredUnicasts = unicasts
	}
}

// WithPeerManagerFilters sets a list of p2p.PeerFilter funcs that are used to
// filter out peers provided by the peer manager PeersProvider.
func WithPeerManagerFilters(peerManagerFilters []p2p.PeerFilter) OptionFn {
	return func(mw *Middleware) {
		mw.peerManagerFilters = peerManagerFilters
	}
}

// WithUnicastRateLimiters sets the unicast rate limiters.
func WithUnicastRateLimiters(rateLimiters *ratelimit.RateLimiters) OptionFn {
	return func(mw *Middleware) {
		mw.unicastRateLimiters = rateLimiters
	}
}

// Config is the configuration for the middleware.
type Config struct {
	Logger                zerolog.Logger
	Libp2pNode            p2p.LibP2PNode
	FlowId                flow.Identifier // This node's Flow ID
	BitSwapMetrics        module.BitswapMetrics
	RootBlockID           flow.Identifier
	UnicastMessageTimeout time.Duration
	IdTranslator          p2p.IDTranslator
	Codec                 network.Codec
}

// Validate validates the configuration, and sets default values for any missing fields.
func (cfg *Config) Validate() {
	if cfg.UnicastMessageTimeout <= 0 {
		cfg.UnicastMessageTimeout = DefaultUnicastTimeout
	}
}

// NewMiddleware creates a new middleware instance
// libP2PNodeFactory is the factory used to create a LibP2PNode
// flowID is this node's Flow ID
// metrics is the interface to report network related metrics
// unicastMessageTimeout is the timeout used for unicast messages
// connectionGating if set to True, restricts this node to only talk to other nodes which are part of the identity list
// validators are the set of the different message validators that each inbound messages is passed through
// During normal operations any error returned by Middleware.start is considered to be catastrophic
// and will be thrown by the irrecoverable.SignalerContext causing the node to crash.
func NewMiddleware(cfg *Config, opts ...OptionFn) *Middleware {
	cfg.Validate()

	// create the node entity and inject dependencies & config
	mw := &Middleware{
		log:                   cfg.Logger,
		libP2PNode:            cfg.Libp2pNode,
		bitswapMetrics:        cfg.BitSwapMetrics,
		rootBlockID:           cfg.RootBlockID,
		validators:            DefaultValidators(cfg.Logger, cfg.FlowId),
		unicastMessageTimeout: cfg.UnicastMessageTimeout,
		idTranslator:          cfg.IdTranslator,
		codec:                 cfg.Codec,
		unicastRateLimiters:   ratelimit.NoopRateLimiters(),
	}

	for _, opt := range opts {
		opt(mw)
	}

	builder := component.NewComponentManagerBuilder()
	for _, limiter := range mw.unicastRateLimiters.Limiters() {
		rateLimiter := limiter
		builder.AddWorker(func(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
			ready()
			rateLimiter.Start(ctx)
			<-rateLimiter.Ready()
			ready()
			<-rateLimiter.Done()
		})
	}
	builder.AddWorker(func(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
		mw.ctx = ctx
		if mw.ov == nil {
			ctx.Throw(fmt.Errorf("overlay has not been set"))
		}

		mw.authorizedSenderValidator = validator.NewAuthorizedSenderValidator(
			mw.log,
			mw.slashingViolationsConsumer,
			mw.ov.Identity)

		err := mw.libP2PNode.WithDefaultUnicastProtocol(mw.handleIncomingStream, mw.preferredUnicasts)
		if err != nil {
			ctx.Throw(fmt.Errorf("could not register preferred unicast protocols on libp2p node: %w", err))
		}

		mw.UpdateNodeAddresses()
		mw.libP2PNode.WithPeersProvider(mw.authorizedPeers)

		ready()

		<-ctx.Done()
		mw.log.Info().Str("component", "middleware").Msg("stopping subroutines")

		// wait for the readConnection and readSubscription routines to stop
		mw.wg.Wait()
		mw.log.Info().Str("component", "middleware").Msg("stopped subroutines")
	})

	mw.Component = builder.Build()
	return mw
}

func DefaultValidators(log zerolog.Logger, flowID flow.Identifier) []network.MessageValidator {
	return []network.MessageValidator{
		validator.ValidateNotSender(flowID),   // validator to filter out messages sent by this node itself
		validator.ValidateTarget(log, flowID), // validator to filter out messages not intended for this node
	}
}

// isProtocolParticipant returns a PeerFilter that returns true if a peer is a staked node.
func (m *Middleware) isProtocolParticipant() p2p.PeerFilter {
	return func(p peer.ID) error {
		if _, ok := m.ov.Identity(p); !ok {
			return fmt.Errorf("failed to get identity of unknown peer with peer id %s", p.String())
		}
		return nil
	}
}

func (m *Middleware) NewBlobService(channel channels.Channel, ds datastore.Batching, opts ...network.BlobServiceOption) network.BlobService {
	return blob.NewBlobService(m.libP2PNode.Host(), m.libP2PNode.Routing(), channel.String(), ds, m.bitswapMetrics, m.log, opts...)
}

func (m *Middleware) NewPingService(pingProtocol protocol.ID, provider network.PingInfoProvider) network.PingService {
	return ping.NewPingService(m.libP2PNode.Host(), pingProtocol, m.log, provider)
}

func (m *Middleware) peerIDs(flowIDs flow.IdentifierList) peer.IDSlice {
	result := make([]peer.ID, 0, len(flowIDs))

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

func (m *Middleware) UpdateNodeAddresses() {
	m.log.Info().Msg("Updating protocol state node addresses")

	ids := m.ov.Identities()
	newInfos, invalid := utils.PeerInfosFromIDs(ids)

	for id, err := range invalid {
		m.log.Err(err).Str("node_id", id.String()).Msg("failed to extract peer info from identity")
	}

	m.Lock()
	defer m.Unlock()

	// set old addresses to expire
	for _, oldInfo := range m.previousProtocolStatePeers {
		m.libP2PNode.Host().Peerstore().SetAddrs(oldInfo.ID, oldInfo.Addrs, peerstore.TempAddrTTL)
	}

	for _, info := range newInfos {
		m.libP2PNode.Host().Peerstore().SetAddrs(info.ID, info.Addrs, peerstore.PermanentAddrTTL)
	}

	m.previousProtocolStatePeers = newInfos
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
func (m *Middleware) SendDirect(msg *network.OutgoingMessageScope) error {
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
	tag := fmt.Sprintf("%v:%v", msg.Channel(), msg.PayloadType())
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
		topic := channels.TopicFromChannel(channel, m.rootBlockID)

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
			network.MessageType(msg.Payload),
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

	topic := channels.TopicFromChannel(channel, m.rootBlockID)

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
	topic := channels.TopicFromChannel(channel, m.rootBlockID)
	err := m.libP2PNode.UnSubscribe(topic)
	if err != nil {
		return fmt.Errorf("failed to unsubscribe from channel (%s): %w", channel, err)
	}

	// update peers to remove nodes subscribed to channel
	m.libP2PNode.RequestPeerUpdate()

	return nil
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

	scope, err := network.NewIncomingScope(originId, protocol, msg, decodedMsgPayload)
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
func (m *Middleware) processMessage(scope *network.IncomingMessageScope) {
	logger := m.log.With().
		Str("channel", scope.Channel().String()).
		Str("type", scope.Protocol().String()).
		Int("msg_size", scope.Size()).
		Hex("origin_id", logging.ID(scope.OriginId())).
		Logger()

	// run through all the message validators
	for _, v := range m.validators {
		// if any one fails, stop message propagation
		if !v.Validate(*scope) {
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
func (m *Middleware) Publish(msg *network.OutgoingMessageScope) error {
	m.log.Debug().
		Str("channel", msg.Channel().String()).
		Interface("msg", msg.Proto()).
		Str("type", msg.PayloadType()).
		Int("msg_size", msg.Size()).
		Msg("publishing new message")

	// convert the message to bytes to be put on the wire.
	data, err := msg.Proto().Marshal()
	if err != nil {
		return fmt.Errorf("failed to marshal the message: %w", err)
	}

	msgSize := len(data)
	if msgSize > p2pnode.DefaultMaxPubSubMsgSize {
		// libp2p pubsub will silently drop the message if its size is greater than the configured pubsub max message size
		// hence return an error as this message is undeliverable
		return fmt.Errorf("message size %d exceeds configured max message size %d", msgSize, p2pnode.DefaultMaxPubSubMsgSize)
	}

	topic := channels.TopicFromChannel(msg.Channel(), m.rootBlockID)

	// publish the bytes on the topic
	err = m.libP2PNode.Publish(m.ctx, topic, data)
	if err != nil {
		return fmt.Errorf("failed to publish the message: %w", err)
	}

	return nil
}

// IsConnected returns true if this node is connected to the node with id nodeID.
// All errors returned from this function can be considered benign.
func (m *Middleware) IsConnected(nodeID flow.Identifier) (bool, error) {
	peerID, err := m.idTranslator.GetPeerID(nodeID)
	if err != nil {
		return false, fmt.Errorf("could not find peer id for target id: %w", err)
	}
	return m.libP2PNode.IsConnected(peerID)
}

// unicastMaxMsgSize returns the max permissible size for a unicast message
func unicastMaxMsgSize(messageType string) int {
	switch messageType {
	case "*messages.ChunkDataResponse":
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
	case "messages.ChunkDataResponse":
		if LargeMsgUnicastTimeout > m.unicastMessageTimeout {
			return LargeMsgUnicastTimeout
		}
		return m.unicastMessageTimeout
	default:
		return m.unicastMessageTimeout
	}
}
