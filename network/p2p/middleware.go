// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package p2p

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"strings"
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
	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/network/message"
	"github.com/onflow/flow-go/network/p2p/unicast"
	"github.com/onflow/flow-go/network/validator"
	psValidator "github.com/onflow/flow-go/network/validator/pubsub"
	_ "github.com/onflow/flow-go/utils/binstat"
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

	// TODO: use this for execution and verification nodes
	// TODO: remove this once we've transitioned to Bitswap for Chunk Data Packs
	// defines maximum message size in unicast mode for large messages
	LargeMsgMaxUnicastMsgSize = gb // 1 gb

	// default maximum time to wait for a default unicast request to complete
	// assuming at least a 1mb/sec connection
	DefaultUnicastTimeout = 5 * time.Second

	// maximum time to wait for a unicast request to complete for large message size
	LargeMsgUnicastTimeout = 1000 * time.Second
)

var _ network.Middleware = (*Middleware)(nil)

// Middleware handles the input & output on the direct connections we have to
// our neighbours on the peer-to-peer network.
type Middleware struct {
	sync.Mutex
	ctx                        context.Context
	log                        zerolog.Logger
	ov                         network.Overlay
	wg                         *sync.WaitGroup
	libP2PNode                 *Node
	libP2PNodeFactory          LibP2PFactoryFunc
	preferredUnicasts          []unicast.ProtocolName
	me                         flow.Identifier
	metrics                    module.NetworkMetrics
	rootBlockID                flow.Identifier
	validators                 []network.MessageValidator
	peerManagerFactory         PeerManagerFactoryFunc
	peerManager                *PeerManager
	unicastMessageTimeout      time.Duration
	connectionGating           bool
	idTranslator               IDTranslator
	previousProtocolStatePeers []peer.AddrInfo
	*component.ComponentManager
}

type MiddlewareOption func(*Middleware)

func WithMessageValidators(validators ...network.MessageValidator) MiddlewareOption {
	return func(mw *Middleware) {
		mw.validators = validators
	}
}

func WithPreferredUnicastProtocols(unicasts []unicast.ProtocolName) MiddlewareOption {
	return func(mw *Middleware) {
		mw.preferredUnicasts = unicasts
	}
}

func WithPeerManager(peerManagerFunc PeerManagerFactoryFunc) MiddlewareOption {
	return func(mw *Middleware) {
		mw.peerManagerFactory = peerManagerFunc
	}
}

func WithConnectionGating(enabled bool) MiddlewareOption {
	return func(mw *Middleware) {
		mw.connectionGating = enabled
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
	rootBlockID flow.Identifier,
	unicastMessageTimeout time.Duration,
	idTranslator IDTranslator,
	opts ...MiddlewareOption,
) *Middleware {

	if unicastMessageTimeout <= 0 {
		unicastMessageTimeout = DefaultUnicastTimeout
	}

	// create the node entity and inject dependencies & config
	mw := &Middleware{
		log:                   log,
		wg:                    &sync.WaitGroup{},
		me:                    flowID,
		libP2PNodeFactory:     libP2PNodeFactory,
		metrics:               metrics,
		rootBlockID:           rootBlockID,
		validators:            DefaultValidators(log, flowID),
		unicastMessageTimeout: unicastMessageTimeout,
		connectionGating:      false,
		peerManagerFactory:    nil,
		idTranslator:          idTranslator,
	}

	for _, opt := range opts {
		opt(mw)
	}

	mw.ComponentManager = component.NewComponentManagerBuilder().
		AddWorker(func(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
			// TODO: refactor to avoid storing ctx altogether
			mw.ctx = ctx

			if err := mw.start(ctx); err != nil {
				ctx.Throw(err)
			}

			ready()

			<-ctx.Done()
			mw.stop()
		}).Build()

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
	return m.peerIDs(m.ov.Identities().NodeIDs())
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

// Me returns the flow identifier of this middleware
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

func (m *Middleware) SetOverlay(ov network.Overlay) {
	m.ov = ov
}

// start will start the middleware.
func (m *Middleware) start(ctx context.Context) error {
	if m.ov == nil {
		return errors.New("overlay must be configured by calling SetOverlay before middleware can be started")
	}

	libP2PNode, err := m.libP2PNodeFactory(ctx)
	if err != nil {
		return fmt.Errorf("could not create libp2p node: %w", err)
	}

	m.libP2PNode = libP2PNode
	err = m.libP2PNode.WithDefaultUnicastProtocol(m.handleIncomingStream, m.preferredUnicasts)
	if err != nil {
		return fmt.Errorf("could not register preferred unicast protocols on libp2p node: %w", err)
	}

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

// stop will end the execution of the middleware and wait for it to end.
func (m *Middleware) stop() {
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

	// wait for the readConnection and readSubscription routines to stop
	m.wg.Wait()
}

func (m *Middleware) writeMessage(stream libp2pnetwork.Stream, msg *message.DirectMessage) error {
	stream.SetWriteDeadline(time.Now().Add(m.unicastMessageTimeout))

	// create a gogo protobuf writer
	bufw := bufio.NewWriter(stream)
	writer := ggio.NewDelimitedWriter(bufw)

	err := writer.WriteMsg(msg)
	if err != nil {
		return fmt.Errorf("failed to write to stream: %w", err)
	}

	// flush the stream
	err = bufw.Flush()
	if err != nil {
		return fmt.Errorf("failed to flush stream: %w", err)
	}

	return nil
}

func (m *Middleware) readMessage(stream libp2pnetwork.Stream) (*message.DirectMessage, error) {
	stream.SetReadDeadline(time.Now().Add(m.unicastMessageTimeout))

	r := ggio.NewDelimitedReader(stream, LargeMsgMaxUnicastMsgSize)

	var msg message.DirectMessage

	// read the next message
	err := r.ReadMsg(&msg)

	if err != nil {
		return nil, err
	}

	return &msg, nil
}

func (m *Middleware) withStream(ctx context.Context, channel network.Channel, target peer.ID, f func(libp2pnetwork.Stream) error) (err error) {
	// TODO: how to tag the connection? maybe use a unique ID for each message?
	tag := fmt.Sprintf("%v:%v", channel, msg.Type)

	m.libP2PNode.connMgr.Protect(target, tag)
	defer m.libP2PNode.connMgr.Unprotect(target, tag)

	var stream libp2pnetwork.Stream

	// TODO: get protocol from channel
	stream, err = m.libP2PNode.CreateStream(ctx, channel, target)
	if err != nil {
		err = fmt.Errorf("failed to create stream for %s: %w", target, err)
		return
	}

	// TODO: the whole point of deferring instead of just doing this after is that it covers panics.
	// In that case, we should not let the decision between reset vs not be based on whether the error
	// is nil or not, because it could panic. if it panics, we should treat that as a reset.
	defer func() {
		if err != nil {
			resetErr := stream.Reset()

			if resetErr != nil {
				m.log.Err(resetErr).Msg("failed to reset stream")
			}

			return
		}

		err = stream.Close()

		if err != nil {
			err = fmt.Errorf("failed to close stream: %w", err)
		}
	}()

	err = f(stream)

	return
}

// SendDirect sends msg on a 1-1 direct connection to the target ID. It models a guaranteed delivery asynchronous
// direct one-to-one connection on the underlying network. No intermediate node on the overlay is utilized
// as the router.
func (m *Middleware) SendDirect(channel network.Channel, msg *message.DirectMessage, target peer.ID) error {
	return m.withStream(m.ctx, channel, target, func(s libp2pnetwork.Stream) error {
		return m.writeMessage(s, msg)
	})
}

func (m *Middleware) SendRequest(
	channel network.Channel,
	msg *message.DirectMessage,
	target peer.ID,
) (*message.DirectMessage, error) {
	var resp *message.DirectMessage

	if err := m.withStream(m.ctx, channel, target, func(s libp2pnetwork.Stream) error {
		err := m.writeMessage(s, msg)

		if err != nil {
			return err
		}

		resp, err = m.readMessage(s)

		if err != nil {
			return err
		}

		return nil
	}); err != nil {
		return nil, err
	}

	return resp, nil
}

func (m *Middleware) directMessageHandler(channel network.Channel) libp2pnetwork.StreamHandler {
	return func(s libp2pnetwork.Stream) {
		success := false

		// qualify the logger with local and remote address
		log := streamLogger(m.log, s)

		log.Info().Msg("incoming stream received")

		defer func() {
			if !success {
				resetErr := s.Reset()

				if resetErr != nil {
					log.Err(resetErr).Msg("failed to reset stream")
				}

				return
			}

			err := s.Close()

			if err != nil {
				log.Err(err).Msg("failed to close stream")
			}
		}()

		sender := s.Conn().RemotePeer()

		msg, err := m.readMessage(s)

		if err != nil {
			log.Err(err).Msg("failed to read message")

			return
		}

		// TODO: metrics for 1to1
		// rc.metrics.NetworkMessageReceived(msg.Size(), channel, msg.Type)

		m.processMessage(msg.Payload, network.Channel(channel.String()), sender)

		success = true
	}
}

func (m *Middleware) requestHandler(channel network.Channel) libp2pnetwork.StreamHandler {
	return func(s libp2pnetwork.Stream) {
		success := false

		// TODO: can we combine this boilerplate with previous method?

		// qualify the logger with local and remote address
		log := streamLogger(m.log, s)

		log.Info().Msg("incoming stream received")

		defer func() {
			if !success {
				resetErr := s.Reset()

				if resetErr != nil {
					log.Err(resetErr).Msg("failed to reset stream")
				}

				return
			}

			err := s.Close()

			if err != nil {
				log.Err(err).Msg("failed to close stream")
			}
		}()

		sender := s.Conn().RemotePeer()

		msg, err := m.readMessage(s)

		if err != nil {
			log.Err(err).Msg("failed to read message")

			return
		}

		// TODO: metrics for 1to1
		// rc.metrics.NetworkMessageReceived(msg.Size(), channel, msg.Type)

		m.processRequest(msg.Payload, network.Channel(channel.String()), sender)

		success = true
	}
}

func (m *Middleware) handleIncomingStream(handler func(s libp2pnetwork.Stream)) libp2pnetwork.StreamHandler {
	return func(s libp2pnetwork.Stream) {
		// TODO: put all the boilerplate here

	}
}

func (m *Middleware) processRequest(payload []byte, channel network.Channel, sender peer.ID, success func(interface{}), fail func(error)) {
	// TODO: can we combine this boilerplate with processMessage?
	msg, err := m.codec.Decode(payload)

	if err != nil {
		m.log.Err(err).Msg("failed to decode message")
		return
	}

	originID, err := m.idTranslator.GetFlowID(peerID)

	if err != nil {
		m.log.Warn().Err(err).Msgf("received message from unknown peer %v, and was dropped", peerID.String())
		return
	}

	m.log.Debug().
		Str("channel", channel.String()).
		Str("type", strings.TrimLeft(fmt.Sprintf("%T", msg), "*")).
		Str("origin_id", originID.String()).
		Msg("processing new message")

	// run through all the message validators
	for _, v := range m.validators {
		// if any one fails, stop message propagation
		if !v.Validate(channel, msg, originID) {
			return
		}
	}

	// if validation passed, send the message to the overlay
	err = m.ov.Receive(originID, channel, msg)
	if err != nil {
		m.log.Error().Err(err).Msg("could not deliver payload")
	}
}

// Subscribe subscribes the middleware to a channel.
func (m *Middleware) Subscribe(channel network.Channel) error {

	topic := engine.TopicFromChannel(channel, m.rootBlockID)

	var validators []psValidator.MessageValidator
	if !engine.PublicChannels().Contains(channel) {
		// for channels used by the staked nodes, add the topic validator to filter out messages from non-staked nodes
		validators = append(validators, psValidator.StakedValidator(m.ov.Identity))
	}

	s, err := m.libP2PNode.Subscribe(topic, validators...)
	if err != nil {
		return fmt.Errorf("failed to subscribe for channel %s: %w", channel, err)
	}

	// create a new readSubscription with the context of the middleware
	rs := newReadSubscription(m.ctx, s, m.processMessage, m.log, m.metrics)
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

// processMessage processes a message and eventually passes it to the overlay
func (m *Middleware) processMessage(
	payload []byte,
	channel network.Channel,
	peerID peer.ID,
) {
	msg, err := m.codec.Decode(payload)

	if err != nil {
		m.log.Err(err).Msg("failed to decode message")
		return
	}

	originID, err := m.idTranslator.GetFlowID(peerID)

	if err != nil {
		m.log.Warn().Err(err).Msgf("received message from unknown peer %v, and was dropped", peerID.String())
		return
	}

	m.log.Debug().
		Str("channel", channel.String()).
		Str("type", strings.TrimLeft(fmt.Sprintf("%T", msg), "*")).
		Str("origin_id", originID.String()).
		Msg("processing new message")

	// run through all the message validators
	for _, v := range m.validators {
		// if any one fails, stop message propagation
		if !v.Validate(channel, msg, originID) {
			return
		}
	}

	// if validation passed, send the message to the overlay
	err = m.ov.Receive(originID, channel, msg)
	if err != nil {
		m.log.Error().Err(err).Msg("could not deliver payload")
	}
}

// Publish publishes a message on the channel. It models a distributed broadcast where the message is meant for all or
// a many nodes subscribing to the channel. It does not guarantee the delivery though, and operates on a best
// effort.
func (m *Middleware) Publish(msg *message.Message, channel network.Channel) error {
	m.log.Debug().Str("channel", channel.String()).Interface("msg", msg).Msg("publishing new message")

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
