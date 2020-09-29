// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package libp2p

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"sync"
	"time"

	ggio "github.com/gogo/protobuf/io"
	"github.com/libp2p/go-libp2p-core/helpers"
	libp2pnetwork "github.com/libp2p/go-libp2p-core/network"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/network/gossip/libp2p/message"
	"github.com/onflow/flow-go/network/gossip/libp2p/middleware"
	"github.com/onflow/flow-go/network/gossip/libp2p/validators"
)

type communicationMode int

const (
	NoOp communicationMode = iota
	OneToOne
	OneToK
)

const (
	// defines maximum message size in publish and multicast modes
	DefaultMaxPubSubMsgSize = 1 << 21 // 2 mb

	// defines maximum message size in unicast mode
	DefaultMaxUnicastMsgSize = 5 * DefaultMaxPubSubMsgSize // 10 mb
)

// the inbound message queue size for One to One and One to K messages (each)
const InboundMessageQueueSize = 100

// Middleware handles the input & output on the direct connections we have to
// our neighbours on the peer-to-peer network.
type Middleware struct {
	sync.Mutex
	ctx               context.Context
	cancel            context.CancelFunc
	log               zerolog.Logger
	codec             network.Codec
	ov                middleware.Overlay
	wg                *sync.WaitGroup
	libP2PNode        *P2PNode
	stop              chan struct{}
	me                flow.Identifier
	host              string
	port              string
	key               crypto.PrivateKey
	metrics           module.NetworkMetrics
	maxPubSubMsgSize  int // used to define maximum message size in pub/sub
	maxUnicastMsgSize int // used to define maximum message size in unicast mode
	rootBlockID       string
	validators        []validators.MessageValidator
}

// NewMiddleware creates a new middleware instance with the given config and using the
// given codec to encode/decode messages to our peers.
func NewMiddleware(log zerolog.Logger, codec network.Codec, address string, flowID flow.Identifier,
	key crypto.PrivateKey, metrics module.NetworkMetrics, maxUnicastMsgSize int, maxPubSubMsgSize int,
	rootBlockID string, validators ...validators.MessageValidator) (*Middleware, error) {
	ip, port, err := net.SplitHostPort(address)
	if err != nil {
		return nil, err
	}

	p2p := &P2PNode{}
	ctx, cancel := context.WithCancel(context.Background())

	if len(validators) == 0 {
		// add default validators to filter out unwanted messages received by this node
		validators = defaultValidators(log, flowID)
	}

	if maxPubSubMsgSize <= 0 {
		maxPubSubMsgSize = DefaultMaxPubSubMsgSize
	}

	if maxUnicastMsgSize <= 0 {
		maxUnicastMsgSize = DefaultMaxUnicastMsgSize
	}

	// create the node entity and inject dependencies & config
	m := &Middleware{
		ctx:               ctx,
		cancel:            cancel,
		log:               log,
		codec:             codec,
		libP2PNode:        p2p,
		wg:                &sync.WaitGroup{},
		stop:              make(chan struct{}),
		me:                flowID,
		host:              ip,
		port:              port,
		key:               key,
		metrics:           metrics,
		maxPubSubMsgSize:  maxPubSubMsgSize,
		maxUnicastMsgSize: maxUnicastMsgSize,
		rootBlockID:       rootBlockID,
		validators:        validators,
	}

	return m, err
}

func defaultValidators(log zerolog.Logger, flowID flow.Identifier) []validators.MessageValidator {
	return []validators.MessageValidator{
		validators.NewSenderValidator(flowID),      // validator to filter out messages sent by this node itself
		validators.NewTargetValidator(log, flowID), // validator to filter out messages not intended for this node
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

func (m *Middleware) PublicKey() crypto.PublicKey {
	return m.key.PublicKey()
}

// Start will start the middleware.
func (m *Middleware) Start(ov middleware.Overlay) error {

	m.ov = ov

	// get the node identity map from the overlay
	idsMap, err := m.ov.Identity()
	if err != nil {
		return fmt.Errorf("could not get identities: %w", err)
	}

	// derive all node addresses from flow identities. Those node address will serve as the network whitelist
	nodeAddrsWhiteList, err := nodeAddresses(idsMap)
	if err != nil {
		return fmt.Errorf("could not derive list of approved peer list: %w", err)
	}

	// create a discovery object to help libp2p discover peers
	d := NewDiscovery(m.log, m.ov, m.me, m.stop)

	// create PubSub options for libp2p to use
	psOptions := []pubsub.Option{
		// set the discovery object
		pubsub.WithDiscovery(d),
		// skip message signing
		pubsub.WithMessageSigning(false),
		// skip message signature
		pubsub.WithStrictSignatureVerification(false),
		// set max message size limit for 1-k PubSub messaging
		pubsub.WithMaxMessageSize(m.maxPubSubMsgSize),
	}

	nodeAddress := NodeAddress{Name: m.me.String(), IP: m.host, Port: m.port}

	libp2pKey, err := PrivKey(m.key)
	if err != nil {
		return fmt.Errorf("failed to translate Flow key to Libp2p key: %w", err)
	}

	// start the libp2p node
	err = m.libP2PNode.Start(m.ctx,
		nodeAddress,
		m.log, libp2pKey,
		m.handleIncomingStream,
		m.rootBlockID,
		true,
		nodeAddrsWhiteList,
		psOptions...)
	if err != nil {
		return fmt.Errorf("failed to start libp2p node: %w", err)
	}

	// the ip,port may change after libp2p has been started. e.g. 0.0.0.0:0 would change to an actual IP and port
	m.host, m.port, err = m.libP2PNode.GetIPPort()
	if err != nil {
		return fmt.Errorf("failed to find IP and port of the libp2p node: %w", err)
	}

	return nil
}

// Stop will end the execution of the middleware and wait for it to end.
func (m *Middleware) Stop() {
	close(m.stop)

	// stop libp2p
	done, err := m.libP2PNode.Stop()
	if err != nil {
		m.log.Error().Err(err).Msg("stopping failed")
	} else {
		<-done
		m.log.Debug().Msg("node stopped successfully")
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
func (m *Middleware) Send(channelID string, msg *message.Message, targetIDs ...flow.Identifier) error {
	var err error
	mode := m.chooseMode(channelID, msg, targetIDs...)
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
		err = m.Publish(msg, channelID)
	default:
		err = fmt.Errorf("invalid communcation mode: %d", mode)
	}

	if err != nil {
		return fmt.Errorf("failed to send message to %s:%w", targetIDs, err)
	}
	return nil
}

// chooseMode determines the communication mode to use. Currently it only considers the length of the targetIDs.
func (m *Middleware) chooseMode(_ string, _ *message.Message, targetIDs ...flow.Identifier) communicationMode {
	switch len(targetIDs) {
	case 0:
		return NoOp
	case 1:
		return OneToOne
	default:
		return OneToK
	}
}

// Dispatch sends msg on a 1-1 direct connection to the target ID. It models a guaranteed delivery asynchronous
// direct one-to-one connection on the underlying network. No intermediate node on the overlay is utilized
// as the router.
//
// Dispatch should be used whenever guaranteed delivery to a specific target is required. Otherwise, Publish is
// a more efficient candidate.
func (m *Middleware) SendDirect(msg *message.Message, targetID flow.Identifier) error {
	targetAddress, err := m.nodeAddressFromID(targetID)
	if err != nil {
		return err
	}

	if msg.Size() > m.maxUnicastMsgSize {
		// message size goes beyond maximum size that the serializer can handle.
		// proceeding with this message results in closing the connection by the target side, and
		// delivery failure.
		return fmt.Errorf("message size %d exceeds configured max message size %d", msg.Size(), m.maxUnicastMsgSize)
	}

	// create new stream
	// (streams don't need to be reused and are fairly inexpensive to be created for each send.
	// A stream creation does NOT incur an RTT as stream negotiation happens as part of the first message
	// sent out the the receiver
	stream, err := m.libP2PNode.CreateStream(m.ctx, targetAddress)
	if err != nil {
		return fmt.Errorf("failed to create stream for %s :%w", targetAddress.Name, err)
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
		return fmt.Errorf("failed to flush stream for %s: %w", targetID.String(), err)
	}

	// track the number of bytes that will be written to the wire for metrics
	byteCount := bufw.Buffered()

	// flush the stream
	err = bufw.Flush()
	if err != nil {
		return fmt.Errorf("failed to flush stream for %s: %w", targetID.String(), err)
	}

	// close the stream immediately
	go helpers.FullClose(stream)

	// OneToOne communication metrics are reported with topic OneToOne
	m.metrics.NetworkMessageSent(byteCount, metrics.ChannelOneToOne, msg.Type)

	return nil
}

// nodeAddressFromID returns the libp2p.NodeAddress for the given flow.id.
func (m *Middleware) nodeAddressFromID(id flow.Identifier) (NodeAddress, error) {

	// get the node identity map from the overlay
	idsMap, err := m.ov.Identity()
	if err != nil {
		return NodeAddress{}, fmt.Errorf("could not get identities: %w", err)
	}

	// retrieve the flow.Identity for the give flow.ID
	flowIdentity, found := idsMap[id]
	if !found {
		return NodeAddress{}, fmt.Errorf("could not get node identity for %s: %w", id.String(), err)
	}

	return nodeAddressFromIdentity(flowIdentity)
}

// nodeAddressFromIdentity returns the libp2p.NodeAddress for the given flow.identity
func nodeAddressFromIdentity(flowIdentity flow.Identity) (NodeAddress, error) {

	// split the node address into ip and port
	ip, port, err := net.SplitHostPort(flowIdentity.Address)
	if err != nil {
		return NodeAddress{}, fmt.Errorf("could not parse address %s: %w", flowIdentity.Address, err)
	}

	// convert the Flow key to a LibP2P key
	lkey, err := PublicKey(flowIdentity.NetworkPubKey)
	if err != nil {
		return NodeAddress{}, fmt.Errorf("could not convert flow key to libp2p key: %w", err)
	}

	// create a new NodeAddress
	nodeAddress := NodeAddress{Name: flowIdentity.NodeID.String(), IP: ip, Port: port, PubKey: lkey}

	return nodeAddress, nil
}

func nodeAddresses(identityMap map[flow.Identifier]flow.Identity) ([]NodeAddress, error) {
	var nodeAddrs []NodeAddress
	for _, identity := range identityMap {
		nodeAddress, err := nodeAddressFromIdentity(identity)
		if err != nil {
			return nil, err
		}

		nodeAddrs = append(nodeAddrs, nodeAddress)
	}
	return nodeAddrs, nil
}

// handleIncomingStream handles an incoming stream from a remote peer
// it is a callback that gets called for each incoming stream by libp2p with a new stream object
func (m *Middleware) handleIncomingStream(s libp2pnetwork.Stream) {

	// qualify the logger with local and remote address
	log := m.log.With().
		Str("local_addr", s.Conn().LocalMultiaddr().String()).
		Str("remote_addr", s.Conn().RemoteMultiaddr().String()).
		Logger()

	log.Info().Msg("incoming connection established")

	//create a new readConnection with the context of the middleware
	conn := newReadConnection(m.ctx, s, m.processMessage, log, m.metrics, m.maxUnicastMsgSize)

	// kick off the receive loop to continuously receive messages
	m.wg.Add(1)
	go conn.receiveLoop(m.wg)
}

// Subscribe will subscribe the middleware for a topic with the fully qualified channel ID name
func (m *Middleware) Subscribe(channelID string) error {

	topic := engine.FullyQualifiedChannelName(channelID, m.rootBlockID)

	s, err := m.libP2PNode.Subscribe(m.ctx, topic)
	if err != nil {
		return fmt.Errorf("failed to subscribe for channel %s: %w", channelID, err)
	}

	// create a new readSubscription with the context of the middleware
	rs := newReadSubscription(m.ctx, s, m.processMessage, m.log, m.metrics)
	m.wg.Add(1)

	// kick off the receive loop to continuously receive messages
	go rs.receiveLoop(m.wg)

	return nil
}

// Unsubscribe will unsubscribe the middleware for a topic with the fully qualified channel ID name
func (m *Middleware) Unsubscribe(channelID string) error {
	topic := engine.FullyQualifiedChannelName(channelID, m.rootBlockID)
	return m.libP2PNode.UnSubscribe(topic)
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

// Publish publishes msg on the channel. It models a distributed broadcast where the message is meant for all or
// a many nodes subscribing to the channel ID. It does not guarantee the delivery though, and operates on a best
// effort.
func (m *Middleware) Publish(msg *message.Message, channelID string) error {

	// convert the message to bytes to be put on the wire.
	data, err := msg.Marshal()
	if err != nil {
		return fmt.Errorf("failed to marshal the message: %w", err)
	}

	msgSize := len(data)
	if msgSize > m.maxPubSubMsgSize {
		// libp2p pubsub will silently drop the message if its size is greater than the configured pubsub max message size
		// hence return an error as this message is undeliverable
		return fmt.Errorf("message size %d exceeds configured max message size %d", msgSize, m.maxPubSubMsgSize)
	}

	topic := engine.FullyQualifiedChannelName(channelID, m.rootBlockID)

	// publish the bytes on the topic
	err = m.libP2PNode.Publish(m.ctx, topic, data)
	if err != nil {
		return fmt.Errorf("failed to publish the message: %w", err)
	}

	m.metrics.NetworkMessageSent(len(data), channelID, msg.Type)

	return nil
}

// Ping pings the target node and returns the ping RTT or an error
func (m *Middleware) Ping(targetID flow.Identifier) (time.Duration, error) {
	nodeAddress, err := m.nodeAddressFromID(targetID)
	if err != nil {
		return -1, err
	}

	return m.libP2PNode.Ping(m.ctx, nodeAddress)
}

// UpdateAllowList fetches the most recent identity of the nodes from overlay
// and updates the underlying libp2p node.
func (m *Middleware) UpdateAllowList() error {
	// get the node identity map from the overlay
	idsMap, err := m.ov.Identity()
	if err != nil {
		return fmt.Errorf("could not get identities: %w", err)
	}

	// derive all node addresses from flow identities
	nodeAddrsAllowList, err := nodeAddresses(idsMap)
	if err != nil {
		return fmt.Errorf("could not derive list of approved peer list: %w", err)
	}

	err = m.libP2PNode.UpdateAllowlist(nodeAddrsAllowList...)
	if err != nil {
		return fmt.Errorf("failed to update approved peer list: %w", err)
	}

	return nil
}
