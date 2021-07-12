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
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/network/message"
	"github.com/onflow/flow-go/network/validator"
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
	ctx                   context.Context
	cancel                context.CancelFunc
	log                   zerolog.Logger
	ov                    network.Overlay
	wg                    *sync.WaitGroup
	libP2PNode            *Node
	libP2PNodeFactory     LibP2PFactoryFunc
	me                    flow.Identifier
	metrics               module.NetworkMetrics
	rootBlockID           string
	validators            []network.MessageValidator
	peerManager           *PeerManager
	peerUpdateInterval    time.Duration
	unicastMessageTimeout time.Duration
}

// NewMiddleware creates a new middleware instance with the given config and using the
// given codec to encode/decode messages to our peers.
func NewMiddleware(log zerolog.Logger,
	libP2PNodeFactory LibP2PFactoryFunc,
	flowID flow.Identifier,
	metrics module.NetworkMetrics,
	rootBlockID string,
	peerUpdateInterval time.Duration,
	unicastMessageTimeout time.Duration,
	validators ...network.MessageValidator) *Middleware {

	if len(validators) == 0 {
		// add default validators to filter out unwanted messages received by this node
		validators = DefaultValidators(log, flowID)
	}

	ctx, cancel := context.WithCancel(context.Background())

	if unicastMessageTimeout <= 0 {
		unicastMessageTimeout = DefaultUnicastTimeout
	}

	// create the node entity and inject dependencies & config
	return &Middleware{
		ctx:                   ctx,
		cancel:                cancel,
		log:                   log,
		wg:                    &sync.WaitGroup{},
		me:                    flowID,
		libP2PNodeFactory:     libP2PNodeFactory,
		metrics:               metrics,
		rootBlockID:           rootBlockID,
		validators:            validators,
		peerUpdateInterval:    peerUpdateInterval,
		unicastMessageTimeout: unicastMessageTimeout,
	}
}

func DefaultValidators(log zerolog.Logger, flowID flow.Identifier) []network.MessageValidator {
	return []network.MessageValidator{
		validator.NewSenderValidator(flowID),      // validator to filter out messages sent by this node itself
		validator.NewTargetValidator(log, flowID), // validator to filter out messages not intended for this node
	}
}

// Me returns the flow identifier of the this middleware
func (m *Middleware) Me() flow.Identifier {
	return m.me
}

// GetIPPort returns the ip address and port number associated with the middleware
func (m *Middleware) GetIPPort() (string, string, error) {
	return m.libP2PNode.GetIPPort()
}

// Start will start the middleware.
func (m *Middleware) Start(ov network.Overlay) error {
	m.ov = ov
	libP2PNode, err := m.libP2PNodeFactory()

	if err != nil {
		return fmt.Errorf("could not create libp2p node: %w", err)
	}
	m.libP2PNode = libP2PNode
	m.libP2PNode.SetFlowProtocolStreamHandler(m.handleIncomingStream)

	// get the node identity map from the overlay
	idsMap, err := m.ov.Identity()
	if err != nil {
		return fmt.Errorf("could not get identities: %w", err)
	}

	err = m.libP2PNode.UpdateAllowList(identityList(idsMap))
	if err != nil {
		return fmt.Errorf("could not update approved peer list: %w", err)
	}

	libp2pConnector, err := newLibp2pConnector(m.libP2PNode.Host(), m.log)
	if err != nil {
		return fmt.Errorf("failed to create libp2pConnector: %w", err)
	}

	m.peerManager = NewPeerManager(m.log, m.ov.Topology, libp2pConnector, WithInterval(m.peerUpdateInterval))
	select {
	case <-m.peerManager.Ready():
		m.log.Debug().Msg("peer manager successfully started")
	case <-time.After(30 * time.Second):
		return fmt.Errorf("could not start peer manager")
	}

	return nil
}

// Stop will end the execution of the middleware and wait for it to end.
func (m *Middleware) Stop() {
	// stops peer manager
	<-m.peerManager.Done()
	m.log.Debug().Msg("peer manager successfully stopped")

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

// Send sends the message to the set of target ids
// If there is only one target NodeID, then a direct 1-1 connection is used by calling middleware.SendDirect
// Otherwise, middleware.Publish is used, which uses the PubSub method of communication.
//
// Deprecated: Send exists for historical compatibility, and should not be used on new
// developments. It is planned to be cleaned up in near future. Proper utilization of Dispatch or
// Publish are recommended instead.
func (m *Middleware) Send(channel network.Channel, msg *message.Message, targetIDs ...flow.Identifier) error {
	var err error
	mode := m.chooseMode(channel, msg, targetIDs...)
	// decide what mode of communication to use
	switch mode {
	case NoOp:
		// NOTE: we can't error on this at the moment, because single nodes of
		// a role in tests will attempt to send messages like this, which should
		// be a no-op, but not an error
		m.log.Debug().Msg("send to no-one")
		return nil
	case OneToOne:
		if targetIDs[0] == m.me {
			// to avoid self dial by the underlay
			m.log.Debug().Msg("send to self")
			return nil
		}
		err = m.SendDirect(msg, targetIDs[0])
	case OneToK:
		err = m.Publish(msg, channel)
	default:
		err = fmt.Errorf("invalid communcation mode: %d", mode)
	}

	if err != nil {
		return fmt.Errorf("failed to send message to %s:%w", targetIDs, err)
	}
	return nil
}

// chooseMode determines the communication mode to use. Currently it only considers the length of the targetIDs.
func (m *Middleware) chooseMode(_ network.Channel, _ *message.Message, targetIDs ...flow.Identifier) communicationMode {
	switch len(targetIDs) {
	case 0:
		return NoOp
	case 1:
		return OneToOne
	default:
		return OneToK
	}
}

// SendDirect sends msg on a 1-1 direct connection to the target ID. It models a guaranteed delivery asynchronous
// direct one-to-one connection on the underlying network. No intermediate node on the overlay is utilized
// as the router.
//
// Dispatch should be used whenever guaranteed delivery to a specific target is required. Otherwise, Publish is
// a more efficient candidate.
func (m *Middleware) SendDirect(msg *message.Message, targetID flow.Identifier) error {
	// translates identifier to identity
	targetIdentity, err := m.identity(targetID)
	if err != nil {
		return fmt.Errorf("could not find identity for target id: %w", err)
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
	stream, err := m.libP2PNode.CreateStream(ctx, targetIdentity)
	if err != nil {
		return fmt.Errorf("failed to create stream for %s :%w", targetID.String(), err)
	}

	// create a gogo protobuf writer
	bufw := bufio.NewWriter(stream)
	writer := ggio.NewDelimitedWriter(bufw)

	err = writer.WriteMsg(msg)
	if err != nil {
		return fmt.Errorf("failed to send message to %s: %w", targetID.String(), err)
	}

	// flush the stream
	err = bufw.Flush()
	if err != nil {
		return fmt.Errorf("failed to flush stream for %s: %w", targetIdentity.String(), err)
	}

	// close the stream immediately
	err = stream.Close()
	if err != nil {
		return fmt.Errorf("failed to close the stream for %s: %w", targetIdentity.String(), err)
	}

	// OneToOne communication metrics are reported with topic OneToOne
	m.metrics.NetworkMessageSent(msg.Size(), metrics.ChannelOneToOne, msg.Type)

	return nil
}

// identity returns corresponding identity of an identifier based on overlay identity list.
func (m *Middleware) identity(identifier flow.Identifier) (flow.Identity, error) {
	// get the node identity map from the overlay
	idsMap, err := m.ov.Identity()
	if err != nil {
		return flow.Identity{}, fmt.Errorf("could not get identities: %w", err)
	}

	// retrieve the flow.Identity for the give flow.ID
	flowIdentity, found := idsMap[identifier]
	if !found {
		return flow.Identity{}, fmt.Errorf("could not get node identity for %s: %w", identifier.String(), err)
	}

	return flowIdentity, nil
}

// identityList translates an identity map into an identity list.
func identityList(identityMap map[flow.Identifier]flow.Identity) flow.IdentityList {
	var identities flow.IdentityList
	for _, identity := range identityMap {
		// casts identity into a local variable to
		// avoid shallow copy of the loop variable
		id := identity
		identities = append(identities, &id)

	}
	return identities
}

// handleIncomingStream handles an incoming stream from a remote peer
// it is a callback that gets called for each incoming stream by libp2p with a new stream object
func (m *Middleware) handleIncomingStream(s libp2pnetwork.Stream) {

	// qualify the logger with local and remote address
	log := streamLogger(m.log, s)

	log.Info().Msg("incoming stream received")

	//create a new readConnection with the context of the middleware
	conn := newReadConnection(m.ctx, s, m.processMessage, log, m.metrics, LargeMsgMaxUnicastMsgSize)

	// kick off the receive loop to continuously receive messages
	m.wg.Add(1)
	go conn.receiveLoop(m.wg)
}

// Subscribe subscribes the middleware to a channel.
func (m *Middleware) Subscribe(channel network.Channel) error {

	topic := engine.TopicFromChannel(channel, m.rootBlockID)

	s, err := m.libP2PNode.Subscribe(m.ctx, topic)
	if err != nil {
		return fmt.Errorf("failed to subscribe for channel %s: %w", channel, err)
	}

	// create a new readSubscription with the context of the middleware
	rs := newReadSubscription(m.ctx, s, m.processMessage, m.log, m.metrics)
	m.wg.Add(1)

	// kick off the receive loop to continuously receive messages
	go rs.receiveLoop(m.wg)

	// update peers to add some nodes interested in the same topic as direct peers
	m.peerManager.RequestPeerUpdate()

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
	m.peerManager.RequestPeerUpdate()

	return nil
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
	data, err := msg.Marshal()
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
	targetIdentity, err := m.identity(targetID)
	if err != nil {
		return message.PingResponse{}, -1, fmt.Errorf("could not find identity for target id: %w", err)
	}

	return m.libP2PNode.Ping(m.ctx, targetIdentity)
}

// UpdateAllowList fetches the most recent identity of the nodes from overlay
// and updates the underlying libp2p node.
func (m *Middleware) UpdateAllowList() error {
	// get the node identity map from the overlay
	idsMap, err := m.ov.Identity()
	if err != nil {
		return fmt.Errorf("could not get identities: %w", err)
	}

	// update libp2pNode's approve lists
	err = m.libP2PNode.UpdateAllowList(identityList(idsMap))
	if err != nil {
		return fmt.Errorf("failed to update approved peer list: %w", err)
	}

	// update peer connections
	m.peerManager.RequestPeerUpdate()

	return nil
}

// IsConnected returns true if this node is connected to the node with id nodeID.
func (m *Middleware) IsConnected(identity flow.Identity) (bool, error) {
	return m.libP2PNode.IsConnected(identity)
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
