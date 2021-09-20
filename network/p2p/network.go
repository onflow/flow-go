package p2p

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/crypto/hash"
	channels "github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/id"
	"github.com/onflow/flow-go/module/lifecycle"
	"github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/network/message"
	"github.com/onflow/flow-go/network/queue"
	_ "github.com/onflow/flow-go/utils/binstat"
)

const DefaultCacheSize = 10e6

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
	identityProvider id.IdentityProvider
	logger           zerolog.Logger
	codec            network.Codec
	me               module.Local
	mw               network.Middleware
	top              network.Topology // used to determine fanout connections
	metrics          module.NetworkMetrics
	rcache           *RcvCache // used to deduplicate incoming messages
	queue            network.MessageQueue
	ctx              context.Context
	cancel           context.CancelFunc
	subMngr          network.SubscriptionManager // used to keep track of subscribed channels
	lifecycleManager *lifecycle.LifecycleManager // used to manage the network's start-stop lifecycle
}

// NewNetwork creates a new naive overlay network, using the given middleware to
// communicate to direct peers, using the given codec for serialization, and
// using the given state & cache interfaces to track volatile information.
// csize determines the size of the cache dedicated to keep track of received messages
func NewNetwork(
	log zerolog.Logger,
	codec network.Codec,
	me module.Local,
	mw network.Middleware,
	csize int,
	top network.Topology,
	sm network.SubscriptionManager,
	metrics module.NetworkMetrics,
	identityProvider id.IdentityProvider,
) (*Network, error) {

	rcache, err := newRcvCache(csize)
	if err != nil {
		return nil, fmt.Errorf("could not initialize cache: %w", err)
	}

	o := &Network{
		logger:           log,
		codec:            codec,
		me:               me,
		mw:               mw,
		rcache:           rcache,
		top:              top,
		metrics:          metrics,
		subMngr:          sm,
		lifecycleManager: lifecycle.NewLifecycleManager(),
		identityProvider: identityProvider,
	}
	o.ctx, o.cancel = context.WithCancel(context.Background())

	// setup the message queue
	// create priority queue
	o.queue = queue.NewMessageQueue(o.ctx, queue.GetEventPriority, metrics)

	// create workers to read from the queue and call queueSubmitFunc
	queue.CreateQueueWorkers(o.ctx, queue.DefaultNumWorkers, o.queue, o.queueSubmitFunc)

	return o, nil
}

// Ready returns a channel that will close when the network stack is ready.
func (n *Network) Ready() <-chan struct{} {
	n.lifecycleManager.OnStart(func() {
		err := n.mw.Start(n)
		if err != nil {
			n.logger.Fatal().Err(err).Msg("failed to start middleware")
		}
	})
	return n.lifecycleManager.Started()
}

// Done returns a channel that will close when shutdown is complete.
func (n *Network) Done() <-chan struct{} {
	n.cancel()
	n.lifecycleManager.OnStop(n.mw.Stop)
	return n.lifecycleManager.Stopped()
}

// Register will register the given engine with the given unique engine engineID,
// returning a conduit to directly submit messages to the message bus of the
// engine.
func (n *Network) Register(channel network.Channel, engine network.Engine) (network.Conduit, error) {
	if !channels.Exists(channel) {
		return nil, fmt.Errorf("unknown channel: %s, should be registered in topic map", channel)
	}

	err := n.subMngr.Register(channel, engine)
	if err != nil {
		return nil, fmt.Errorf("failed to register engine for channel %s: %w", channel, err)
	}

	n.logger.Info().
		Str("channel_id", channel.String()).
		Msg("channel successfully registered")

	// create a cancellable child context
	ctx, cancel := context.WithCancel(n.ctx)

	// create the conduit
	conduit := &Conduit{
		ctx:       ctx,
		cancel:    cancel,
		channel:   channel,
		publish:   n.publish,
		unicast:   n.unicast,
		multicast: n.multicast,
		close:     n.unregister,
	}

	return conduit, nil
}

// unregister unregisters the engine for the specified channel. The engine will no longer be able to send or
// receive messages from that channel
func (n *Network) unregister(channel network.Channel) error {
	err := n.subMngr.Unregister(channel)
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

// Topology returns the identities of a uniform subset of nodes in protocol state using the topology provided earlier.
// Independent invocations of Topology on different nodes collectively constructs a connected network graph.
func (n *Network) Topology() (flow.IdentityList, error) {
	n.Lock()
	defer n.Unlock()

	subscribedChannels := n.subMngr.Channels()
	top, err := n.top.GenerateFanout(n.Identities(), subscribedChannels)
	if err != nil {
		return nil, fmt.Errorf("could not generate topology: %w", err)
	}
	return top, nil
}

func (n *Network) Receive(nodeID flow.Identifier, msg *message.Message) error {
	err := n.processNetworkMessage(nodeID, msg)
	if err != nil {
		return fmt.Errorf("could not process message: %w", err)
	}
	return nil
}

func (n *Network) processNetworkMessage(senderID flow.Identifier, message *message.Message) error {
	// checks the cache for deduplication and adds the message if not already present
	if n.rcache.add(message.EventID, network.Channel(message.ChannelID)) {
		log := n.logger.With().
			Hex("sender_id", senderID[:]).
			Hex("event_id", message.EventID).
			Logger()

		// drops duplicate message
		log.Debug().
			Str("channel", message.ChannelID).
			Msg("dropping message due to duplication")

		n.metrics.NetworkDuplicateMessagesDropped(message.ChannelID, message.Type)

		return nil
	}

	// Convert message payload to a known message type
	decodedMessage, err := n.codec.Decode(message.Payload)
	if err != nil {
		return fmt.Errorf("could not decode event: %w", err)
	}

	// create queue message
	qm := queue.QMessage{
		Payload:  decodedMessage,
		Size:     message.Size(),
		Target:   network.Channel(message.ChannelID),
		SenderID: senderID,
	}

	// insert the message in the queue
	err = n.queue.Insert(qm)
	if err != nil {
		return fmt.Errorf("failed to insert message in queue: %w", err)
	}

	return nil
}

// genNetworkMessage uses the codec to encode an event into a NetworkMessage
func (n *Network) genNetworkMessage(channel network.Channel, event interface{}, targetIDs ...flow.Identifier) (*message.Message, error) {
	// encode the payload using the configured codec
	payload, err := n.codec.Encode(event)
	if err != nil {
		return nil, fmt.Errorf("could not encode event: %w", err)
	}

	//bs := binstat.EnterTimeVal(binstat.BinNet+":wire<3payload2message", int64(len(payload)))
	//defer binstat.Leave(bs)

	// use a hash with an engine-specific salt to get the payload hash
	h := hash.NewSHA3_384()
	_, err = h.Write([]byte("libp2ppacking" + channel))
	if err != nil {
		return nil, fmt.Errorf("could not hash channel as salt: %w", err)
	}

	_, err = h.Write(payload)
	if err != nil {
		return nil, fmt.Errorf("could not hash event: %w", err)
	}

	payloadHash := h.SumHash()

	var emTargets [][]byte
	for _, targetID := range targetIDs {
		tempID := targetID // avoid capturing loop variable
		emTargets = append(emTargets, tempID[:])
	}

	// get origin ID
	selfID := n.me.NodeID()
	originID := selfID[:]

	// get message type from event type and remove the asterisk prefix if present
	msgType := strings.TrimLeft(fmt.Sprintf("%T", event), "*")

	// cast event to a libp2p.Message
	msg := &message.Message{
		ChannelID: channel.String(),
		EventID:   payloadHash,
		OriginID:  originID,
		TargetIDs: emTargets,
		Payload:   payload,
		Type:      msgType,
	}

	return msg, nil
}

// unicast sends the message in a reliable way to the given recipient.
// It uses 1-1 direct messaging over the underlying network to deliver the message.
// It returns an error if unicasting fails.
func (n *Network) unicast(channel network.Channel, message interface{}, targetID flow.Identifier) error {
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

// publish sends the message in an unreliable way to the given recipients.
// In this context, unreliable means that the message is published over a libp2p pub-sub
// channel and can be read by any node subscribed to that channel.
// The selector could be used to optimize or restrict delivery.
func (n *Network) publish(channel network.Channel, message interface{}, targetIDs ...flow.Identifier) error {
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

// multicast unreliably sends the specified event over the channel to randomly selected 'num' number of recipients
// selected from the specified targetIDs.
func (n *Network) multicast(channel network.Channel, message interface{}, num uint, targetIDs ...flow.Identifier) error {
	selectedIDs := flow.IdentifierList(targetIDs).Filter(n.removeSelfFilter()).Sample(num)

	if len(selectedIDs) == 0 {
		return network.EmptyTargetList
	}

	err := n.sendOnChannel(channel, message, targetIDs)

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
func (n *Network) sendOnChannel(channel network.Channel, message interface{}, targetIDs []flow.Identifier) error {
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
	eng, err := n.subMngr.GetEngine(qm.Target)
	if err != nil {
		n.logger.Error().
			Err(err).
			Str("channel_id", qm.Target.String()).
			Str("sender_id", qm.SenderID.String()).
			Msg("failed to submit message")
		return
	}

	// submits the message to the engine synchronously and
	// tracks its processing time.
	startTimestamp := time.Now()

	err = eng.Process(qm.Target, qm.SenderID, qm.Payload)
	if err != nil {
		n.logger.Error().
			Err(err).
			Str("channel_id", qm.Target.String()).
			Str("sender_id", qm.SenderID.String()).
			Msg("failed to process message")
	}

	n.metrics.InboundProcessDuration(qm.Target.String(), time.Since(startTimestamp))
}
