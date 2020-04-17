package libp2p

import (
	"fmt"
	"sync"

	"github.com/dchest/siphash"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/module"
	"github.com/dapperlabs/flow-go/network"
	"github.com/dapperlabs/flow-go/network/gossip/libp2p/cache"
	libp2perrors "github.com/dapperlabs/flow-go/network/gossip/libp2p/errors"
	"github.com/dapperlabs/flow-go/network/gossip/libp2p/message"
	"github.com/dapperlabs/flow-go/network/gossip/libp2p/middleware"
)

// Network represents the overlay network of our peer-to-peer network, including
// the protocols for handshakes, authentication, gossiping and heartbeats.
type Network struct {
	sync.RWMutex
	logger  zerolog.Logger
	codec   network.Codec
	ids     flow.IdentityList
	me      module.Local
	mw      middleware.Middleware
	top     middleware.Topology
	engines map[uint8]network.Engine
	rcache  *cache.RcvCache // used to deduplicate incoming messages
	fanout  int             // used to determine number of nodes' neighbors on overlay
}

// NewNetwork creates a new naive overlay network, using the given middleware to
// communicate to direct peers, using the given codec for serialization, and
// using the given state & cache interfaces to track volatile information.
// csize determines the size of the cache dedicated to keep track of received messages
func NewNetwork(
	log zerolog.Logger,
	codec network.Codec,
	ids flow.IdentityList,
	me module.Local,
	mw middleware.Middleware,
	csize int,
	top middleware.Topology) (*Network, error) {

	rcache, err := cache.NewRcvCache(csize)
	if err != nil {
		return nil, fmt.Errorf("could not initialize cache: %w", err)
	}

	// fanout is set to half of the system size for connectivity assurance w.h.p
	// ids contain the address of this node itself (hence fanout is (len(ids) - 1) + 1) / 2) = len(ids) / 2
	fanout := len(ids) / 2

	o := &Network{
		logger:  log,
		codec:   codec,
		me:      me,
		mw:      mw,
		engines: make(map[uint8]network.Engine),
		rcache:  rcache,
		fanout:  fanout,
		top:     top,
	}

	o.SetIDs(ids)

	return o, nil
}

// Ready returns a channel that will close when the network stack is ready.
func (n *Network) Ready() <-chan struct{} {
	ready := make(chan struct{})
	go func() {
		err := n.mw.Start(n)
		if err != nil {
			n.logger.Err(err).Msg("failed to start middleware")
		}
		close(ready)
	}()
	return ready
}

// Done returns a channel that will close when shutdown is complete.
func (n *Network) Done() <-chan struct{} {
	done := make(chan struct{})
	go func() {
		n.mw.Stop()
		close(done)
	}()
	return done
}

// Register will register the given engine with the given unique engine engineID,
// returning a conduit to directly submit messages to the message bus of the
// engine.
func (n *Network) Register(channelID uint8, engine network.Engine) (network.Conduit, error) {
	n.Lock()
	defer n.Unlock()

	// check if the engine engineID is already taken
	_, ok := n.engines[channelID]
	if ok {
		return nil, fmt.Errorf("engine already registered (%d)", engine)
	}

	// Register the middleware for the channelID topic
	err := n.mw.Subscribe(channelID)
	if err != nil {
		return nil, fmt.Errorf("failed to subscribe to channel %d: %w", channelID, err)
	}

	// create the conduit
	conduit := &Conduit{
		channelID: channelID,
		submit:    n.submit,
	}

	// register engine with provided engineID
	n.engines[channelID] = engine
	return conduit, nil
}

// Identity returns a map of all flow.Identifier to flow identity by querying the flow state
func (n *Network) Identity() (map[flow.Identifier]flow.Identity, error) {
	identifierToID := make(map[flow.Identifier]flow.Identity)
	for _, id := range n.ids {
		identifierToID[id.NodeID] = *id
	}
	return identifierToID, nil
}

// Topology returns the identities of a uniform subset of nodes in protocol state using the topology provided earlier
func (n *Network) Topology() (map[flow.Identifier]flow.Identity, error) {
	return n.top.Subset(n.ids, n.fanout, n.me.NodeID().String())
}

func (n *Network) Receive(nodeID flow.Identifier, msg interface{}) error {

	var err error
	switch m := msg.(type) {
	case *message.Message:
		err = n.processNetworkMessage(nodeID, m)
	default:
		err = fmt.Errorf("network received invalid message type (%T)", m)
	}
	if err != nil {
		err = fmt.Errorf("could not process message: %w", err)
	}
	return err
}

func (n *Network) SetIDs(ids flow.IdentityList) {
	// remove this node id from the list of fanout target ids to avoid self-dial
	idsMinusMe := ids.Filter(n.me.NotMeFilter())
	n.ids = idsMinusMe
}

func (n *Network) processNetworkMessage(senderID flow.Identifier, message *message.Message) error {
	// checks the cache for deduplication
	if n.rcache.Seen(message.EventID, message.ChannelID) {
		// drops duplicate message
		n.logger.Debug().Bytes("event ID", message.EventID).
			Msg(" dropping message due to duplication")
		return nil
	}
	n.rcache.Add(message.EventID, message.ChannelID)

	// Extract channel id and find the registered engine
	channelID := uint8(message.ChannelID)
	en, found := n.engines[channelID]
	if !found {
		return libp2perrors.NewInvalidEngineError(channelID, senderID.String())
	}

	// Convert message payload to a known message type
	decodedMessage, err := n.codec.Decode(message.Payload)
	if err != nil {
		return fmt.Errorf("could not decode event: %w", err)
	}

	// call the engine with the message payload
	err = en.Process(senderID, decodedMessage)
	if err != nil {
		n.logger.Error().Str("sender", senderID.String()).Uint8("channel", channelID).Err(err)
		return fmt.Errorf("failed to process message from %s: %w", senderID.String(), err)
	}
	return nil
}

// genNetworkMessage uses the codec to encode an event into a NetworkMessage
func (n *Network) genNetworkMessage(channelID uint8, event interface{}, targetIDs ...flow.Identifier) (*message.Message, error) {
	// encode the payload using the configured codec
	payload, err := n.codec.Encode(event)
	if err != nil {
		return nil, errors.Wrap(err, "could not encode event")
	}

	// use a hash with an engine-specific salt to get the payload hash
	sip := siphash.New([]byte("libp2ppacking" + fmt.Sprintf("%03d", channelID)))

	var emTargets [][]byte
	for _, t := range targetIDs {
		// create a slice out of the temp var
		emTargets = append(emTargets, t[:])
	}

	// get origin ID (inplace slicing n.me.NodeID()[:] doesn't work)
	selfID := n.me.NodeID()
	originID := selfID[:]

	//cast event to a libp2p.Message
	msg := &message.Message{
		ChannelID: uint32(channelID),
		EventID:   sip.Sum(payload),
		OriginID:  originID,
		TargetIDs: emTargets,
		Payload:   payload,
	}

	return msg, nil
}

// submit method submits the given event for the given channel to the overlay layer
// for processing; it is used by engines through conduits.
func (n *Network) submit(channelID uint8, event interface{}, targetIDs ...flow.Identifier) error {
	// genNetworkMessage the event to get payload and event ID
	msg, err := n.genNetworkMessage(channelID, event, targetIDs...)
	if err != nil {
		return errors.Wrap(err, "could not cast the event into network message")
	}

	// TODO: dedup the message here

	err = n.mw.Send(channelID, msg, targetIDs...)
	if err != nil {
		return errors.Wrap(err, "could not gossip event")
	}

	return nil
}
