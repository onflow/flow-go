// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package p2p

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"strings"
	"sync"
	"time"

	"github.com/ipfs/go-datastore"
	libp2pnetwork "github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/peerstore"
	"github.com/libp2p/go-libp2p-core/protocol"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/network"
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
	ctx context.Context
	log zerolog.Logger
	ov  network.Overlay
	// TODO: using a waitgroup here doesn't actually guarantee that we'll wait for all
	// goroutines to exit, because new goroutines could be started after we've already
	// returned from wg.Wait(). We need to solve this the right way using ComponentManager
	// and worker routines.
	wg                         *sync.WaitGroup
	libP2PNode                 *Node
	libP2PNodeFactory          LibP2PFactoryFunc
	preferredUnicasts          []unicast.ProtocolName
	me                         flow.Identifier
	metrics                    module.NetworkMetrics
	sporkID                    flow.Identifier
	validators                 []network.MessageValidator
	peerManagerFactory         PeerManagerFactoryFunc
	peerManager                *PeerManager
	idTranslator               IDTranslator
	previousProtocolStatePeers []peer.AddrInfo
	directMessageConfigs       map[network.Channel]DirectMessageConfig
	directMessageConfigsLock   sync.RWMutex
	codec                      network.Codec
	component.Component
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
	codec network.Codec,
	flowID flow.Identifier,
	metrics module.NetworkMetrics,
	sporkID flow.Identifier,
	idTranslator IDTranslator,
	opts ...MiddlewareOption,
) *Middleware {

	// create the node entity and inject dependencies & config
	mw := &Middleware{
		log:                  log,
		wg:                   &sync.WaitGroup{},
		me:                   flowID,
		libP2PNodeFactory:    libP2PNodeFactory,
		metrics:              metrics,
		sporkID:              sporkID,
		validators:           DefaultValidators(log, flowID),
		peerManagerFactory:   nil,
		idTranslator:         idTranslator,
		directMessageConfigs: make(map[network.Channel]DirectMessageConfig),
		codec:                codec,
	}

	for _, opt := range opts {
		opt(mw)
	}

	cm := component.NewComponentManagerBuilder().
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

	mw.Component = cm

	return mw
}

func DefaultValidators(log zerolog.Logger, flowID flow.Identifier) []network.MessageValidator {
	return []network.MessageValidator{
		validator.ValidateNotSender(flowID), // validator to filter out messages sent by this node itself
	}
}

func (m *Middleware) NewBlobService(channel network.Channel, ds datastore.Batching, opts ...network.BlobServiceOption) network.BlobService {
	return NewBlobService(m.libP2PNode.Host(), m.libP2PNode.routing, channel.String(), ds, opts...)
}

func (m *Middleware) NewPingService(pingProtocol protocol.ID, provider network.PingInfoProvider) network.PingService {
	return NewPingService(m.libP2PNode.Host(), pingProtocol, m.log, provider)
}

func (m *Middleware) topologyPeers() (peer.IDSlice, error) {
	identities, err := m.ov.Topology()
	if err != nil {
		return nil, err
	}

	return m.peerIDs(identities.NodeIDs()), nil
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

	m.UpdateNodeAddresses()

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

// SendDirect sends msg on a 1-1 direct connection to the target ID. It models a guaranteed delivery asynchronous
// direct one-to-one connection on the underlying network. No intermediate node on the overlay is utilized
// as the router.
//
// Dispatch should be used whenever guaranteed delivery to a specific target is required. Otherwise, Publish is
// a more efficient candidate.
func (m *Middleware) SendDirect(channel network.Channel, event interface{}, targetID flow.Identifier) (err error) {
	// translates identifier to peer id
	peerID, err := m.idTranslator.GetPeerID(targetID)
	if err != nil {
		return fmt.Errorf("could not find peer id for target id: %w", err)
	}

	// encode the payload using the configured codec
	payload, err := m.codec.Encode(event)
	if err != nil {
		return fmt.Errorf("could not encode event: %w", err)
	}

	config := m.getDirectMessageConfig(channel)

	msgSize := len(payload)
	if msgSize > config.MaxMsgSize {
		// message size goes beyond maximum size that the serializer can handle.
		// proceeding with this message results in closing the connection by the target side, and
		// delivery failure.
		return fmt.Errorf("message size %d exceeds configured max message size %d", msgSize, config.MaxMsgSize)
	}

	m.metrics.DirectMessageStarted(channel.String())
	defer m.metrics.DirectMessageFinished(channel.String())

	msgType := m.msgType(event)

	// pass in a context with timeout to make the unicast call fail fast
	ctx, cancel := context.WithTimeout(m.ctx, config.MaxMsgTimeout)
	defer cancel()

	// protect the underlying connection from being inadvertently pruned by the peer manager while the stream and
	// connection creation is being attempted, and remove it from protected list once stream created.
	randID := make([]byte, 8)
	rand.Read(randID)
	tag := fmt.Sprintf("%v:%v:%v", channel, msgType, string(randID))
	m.libP2PNode.host.ConnManager().Protect(peerID, tag)
	defer m.libP2PNode.host.ConnManager().Unprotect(peerID, tag)

	// create new stream
	// (streams don't need to be reused and are fairly inexpensive to be created for each send.
	// A stream creation does NOT incur an RTT as stream negotiation happens as part of the first message
	// sent out the receiver
	stream, err := m.libP2PNode.CreateStream(ctx, peerID)
	if err != nil {
		return fmt.Errorf("failed to create stream for %s: %w", targetID, err)
	}

	success := false

	defer func() {
		if success {
			// close the stream immediately
			err = stream.Close()
			if err != nil {
				err = fmt.Errorf("failed to close the stream for %s: %w", targetID, err)
			}

			// OneToOne communication metrics are reported with topic OneToOne
			m.metrics.NetworkMessageSent(msgSize, metrics.ChannelOneToOne, msgType)
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

	_, err = stream.Write(payload)

	if err != nil {
		return fmt.Errorf("failed to write message to stream: %w", err)
	}

	success = true

	return nil
}

func (m *Middleware) msgType(event interface{}) string {
	return strings.TrimLeft(fmt.Sprintf("%T", event), "*")
}

func (m *Middleware) getDirectMessageConfig(channel network.Channel) DirectMessageConfig {
	m.directMessageConfigsLock.RLock()
	config, ok := m.directMessageConfigs[channel]
	m.directMessageConfigsLock.RUnlock()

	if ok {
		return config
	}

	return DirectMessageConfig{
		MaxMsgSize:    DefaultMaxUnicastMsgSize,
		MaxMsgTimeout: DefaultUnicastTimeout,
	}
}

func (m *Middleware) streamHandler(channel network.Channel, handler network.DirectMessageHandler) libp2pnetwork.StreamHandler {
	return func(stream libp2pnetwork.Stream) {
		// qualify the logger with local and remote address
		log := streamLogger(m.log, stream)

		log.Info().Msg("incoming stream received")

		success := false

		defer func() {
			if success {
				err := stream.Close()
				if err != nil {
					log.Err(err).Msg("failed to close stream")
				}
			} else {
				err := stream.Reset()
				if err != nil {
					log.Err(err).Msg("failed to reset stream")
				}
			}
		}()

		peerID := stream.Conn().RemotePeer()
		flowID, err := m.idTranslator.GetFlowID(peerID)
		if err != nil {
			m.log.Warn().Err(err).Msgf("received message from unknown peer %v, and was dropped", peerID.String())
			return
		}

		config := m.getDirectMessageConfig(channel)

		ctx, cancel := context.WithTimeout(m.ctx, config.MaxMsgTimeout)
		defer cancel()

		deadline, _ := ctx.Deadline()

		err = stream.SetReadDeadline(deadline)
		if err != nil {
			log.Err(err).Msg("failed to set read deadline for stream")
			return
		}

		data, err := io.ReadAll(stream)
		if err != nil {
			m.log.Err(err).Msg("failed to read message from stream:")
			return
		}

		decodedMessage, err := m.codec.Decode(data)
		if err != nil {
			m.log.Err(err).Msg("could not decode msg")
			return
		}

		// log metrics with the channel name as OneToOne
		m.metrics.NetworkMessageReceived(len(data), metrics.ChannelOneToOne, m.msgType(decodedMessage))

		handler(channel, decodedMessage, flowID)

		success = true
	}
}

type DirectMessageConfig struct {
	MaxMsgSize    int
	MaxMsgTimeout time.Duration
}

func (m *Middleware) SetDirectMessageConfig(channel network.Channel, config DirectMessageConfig) {
	m.directMessageConfigsLock.Lock()
	defer m.directMessageConfigsLock.Unlock()

	m.directMessageConfigs[channel] = config
}

func (m *Middleware) SetDirectMessageHandler(channel network.Channel, handler network.DirectMessageHandler) {
	m.libP2PNode.Host().SetStreamHandler(
		protocol.ID(engine.TopicFromChannel(channel, m.sporkID)),
		m.streamHandler(channel, handler),
	)
}

// Subscribe subscribes the middleware to a channel.
func (m *Middleware) Subscribe(channel network.Channel) error {

	topic := engine.TopicFromChannel(channel, m.sporkID)

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
	rs := newReadSubscription(m.ctx, s, m.pubSubMessageHandler(channel), m.log)
	m.wg.Add(1)

	// kick off the receive loop to continuously receive messages
	go rs.receiveLoop(m.wg)

	// update peers to add some nodes interested in the same topic as direct peers
	m.peerManagerUpdate()

	return nil
}

// Unsubscribe unsubscribes the middleware from a channel.
func (m *Middleware) Unsubscribe(channel network.Channel) error {
	topic := engine.TopicFromChannel(channel, m.sporkID)
	err := m.libP2PNode.UnSubscribe(topic)
	if err != nil {
		return fmt.Errorf("failed to unsubscribe from channel %s: %w", channel, err)
	}

	// update peers to remove nodes subscribed to channel
	m.peerManagerUpdate()

	return nil
}

func (m *Middleware) pubSubMessageHandler(channel network.Channel) func(interface{}, int, peer.ID) {
	return func(msg interface{}, rawLength int, peerID peer.ID) {
		// log metrics
		m.metrics.NetworkMessageReceived(rawLength, channel.String(), m.msgType(msg))

		flowID, err := m.idTranslator.GetFlowID(peerID)
		if err != nil {
			m.log.Warn().Err(err).Msgf("received message from unknown peer %v, and was dropped", peerID.String())
			return
		}

		m.log.Debug().
			Str("channel", channel.String()).
			Str("type", m.msgType(msg)).
			Str("origin_id", flowID.String()).
			Msg("processing new message")

		// run through all the message validators
		for _, v := range m.validators {
			// if any one fails, stop message propagation
			if !v.Validate(flowID, msg) {
				return
			}
		}

		// if validation passed, send the message to the overlay
		err = m.ov.Receive(flowID, channel, msg)
		if err != nil {
			m.log.Error().Err(err).Msg("could not deliver payload")
		}
	}
}

// Publish publishes a message on the channel. It models a distributed broadcast where the message is meant for all or
// a many nodes subscribing to the channel. It does not guarantee the delivery though, and operates on a best
// effort.
func (m *Middleware) Publish(event interface{}, channel network.Channel) error {
	m.log.Debug().Str("channel", channel.String()).Interface("msg", event).Msg("publishing new message")

	// encode the payload using the configured codec
	payload, err := m.codec.Encode(event)
	if err != nil {
		return fmt.Errorf("could not encode event: %w", err)
	}

	msgSize := len(payload)
	if msgSize > DefaultMaxPubSubMsgSize {
		// libp2p pubsub will silently drop the message if its size is greater than the configured pubsub max message size
		// hence return an error as this message is undeliverable
		return fmt.Errorf("message size %d exceeds configured max message size %d", msgSize, DefaultMaxPubSubMsgSize)
	}

	topic := engine.TopicFromChannel(channel, m.sporkID)

	// publish the bytes on the topic
	err = m.libP2PNode.Publish(m.ctx, topic, payload)
	if err != nil {
		return fmt.Errorf("failed to publish the message: %w", err)
	}

	m.metrics.NetworkMessageSent(msgSize, string(channel), m.msgType(event))

	return nil
}

// IsConnected returns true if this node is connected to the node with id nodeID.
func (m *Middleware) IsConnected(nodeID flow.Identifier) (bool, error) {
	peerID, err := m.idTranslator.GetPeerID(nodeID)
	if err != nil {
		return false, fmt.Errorf("could not find peer id for target id: %w", err)
	}
	return m.libP2PNode.IsConnected(peerID)
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
