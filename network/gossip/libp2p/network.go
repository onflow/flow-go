package libp2p

import (
	"fmt"
	"hash"
	"math/rand"
	"sync"

	"github.com/dchest/siphash"
	"github.com/hashicorp/go-multierror"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/flow/identity"
	networkmodel "github.com/dapperlabs/flow-go/model/libp2p/network"
	"github.com/dapperlabs/flow-go/module"
	"github.com/dapperlabs/flow-go/network"
	"github.com/dapperlabs/flow-go/network/gossip/libp2p/cache"
	"github.com/dapperlabs/flow-go/network/gossip/libp2p/middleware"
	"github.com/dapperlabs/flow-go/network/gossip/libp2p/topology"
	"github.com/dapperlabs/flow-go/protocol"
)

// Network represents the overlay network of our peer-to-peer network, including
// the protocols for handshakes, authentication, gossiping and heartbeats.
type Network struct {
	sync.RWMutex
	logger  zerolog.Logger
	codec   network.Codec
	state   protocol.State
	me      module.Local
	mw      middleware.Middleware
	cache   *cache.Cache
	top     *topology.Topology
	sip     hash.Hash
	engines map[uint8]network.Engine
}

// NewNetwork creates a new naive overlay network, using the given middleware to
// communicate to direct peers, using the given codec for serialization, and
// using the given state & cache interfaces to track volatile information.
func NewNetwork(log zerolog.Logger, codec network.Codec, state protocol.State, me module.Local, mw middleware.Middleware) (*Network, error) {

	top, err := topology.New()
	if err != nil {
		return nil, errors.Wrap(err, "could not initialize topology")
	}

	ca, err := cache.New()
	if err != nil {
		return nil, errors.Wrap(err, "could not initialize cache")
	}

	o := &Network{
		logger:  log,
		codec:   codec,
		state:   state,
		me:      me,
		mw:      mw,
		top:     top,
		cache:   ca,
		sip:     siphash.New([]byte("daflowtrickleson")),
		engines: make(map[uint8]network.Engine),
	}

	return o, nil
}

// Ready returns a channel that will close when the network stack is ready.
func (n *Network) Ready() <-chan struct{} {
	ready := make(chan struct{})
	n.mw.Start(n)
	go func() {
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
		return nil, errors.Errorf("engine already registered (%d)", engine)
	}

	// add the engine ID to the cache
	n.cache.Add(channelID)

	// create the conduit
	conduit := &Conduit{
		channelID: channelID,
		submit:    n.submit,
	}

	// register engine with provided engineID
	n.engines[channelID] = engine

	return conduit, nil
}

// Address implements a callback to provide the overlay layer with the
// addresses we want to establish new connections to.
func (n *Network) Identity() (flow.Identity, error) {

	// get a list of other nodes that are not us, and we are not connected to
	nodeIDs := n.top.Peers().NodeIDs()
	nodeIDs = append(nodeIDs, n.me.NodeID())
	ids, err := n.state.Final().Identities(identity.Not(identity.HasNodeID(nodeIDs...)))
	if err != nil {
		return flow.Identity{}, errors.Wrap(err, "could not get identities")
	}

	// if we don't have nodes available, we can't do anything
	if len(ids) == 0 {
		return flow.Identity{}, errors.New("no identities available")
	}

	// select a random identity from the list
	id := ids[rand.Int()%len(ids)]

	return id, nil
}

// Cleanup implements a callback to handle peers that have been dropped
// by the middleware layer.
func (n *Network) Cleanup(nodeID flow.Identifier) error {
	// drop the peer state using the ID we registered
	n.top.Down(nodeID)
	return nil
}

func (n *Network) Receive(nodeID flow.Identifier, msg interface{}) error {

	var err error

	switch m := msg.(type) {
	case networkmodel.NetworkMessage:
		err = n.processNetworkMessage(nodeID, m)
	default:
		err = errors.Errorf("invalid message type (%T)", m)
	}
	if err != nil {
		err = errors.Wrap(err, "could not process message")
	}
	return err
}

func (n *Network) processNetworkMessage(nodeID flow.Identifier, message networkmodel.NetworkMessage) error {

	// Extract engine id and find the registered engine
	en, found := n.engines[message.ChannelID]
	if !found {
		n.logger.Debug().Str("sender", nodeID.String()).
			Uint8("engine", uint8(message.ChannelID)).
			Msg(" dropping message since no engine to receive it was found")
		return fmt.Errorf("could not find the engine: %d", message.ChannelID)
	}

	// Convert message payload to a known message type
	payload, err := n.codec.Decode(message.Payload)
	if err != nil {
		return errors.Wrap(err, "could not decode event")
	}

	// call the engine with the message payload
	err = en.Process(message.OriginID, payload)
	if err != nil {
		n.logger.Error().
			Uint8("engineid", message.ChannelID).Str("sender", nodeID.String()).Err(err)
		return err
	}
	return nil
}

// genNetworkMessage uses the codec to encode an event into a NetworkMessage
func (n *Network) genNetworkMessage(channelID uint8, event interface{}, targetIDs ...flow.Identifier) (networkmodel.NetworkMessage, error) {
	// encode the payload using the configured codec
	payload, err := n.codec.Encode(event)
	if err != nil {
		return networkmodel.NetworkMessage{}, errors.Wrap(err, "could not encode event")
	}

	// use a hash with an engine-specific salt to get the payload hash
	sip := siphash.New([]byte("libp2ppacking" + fmt.Sprintf("%03d", channelID)))

	// casting event structure
	e := networkmodel.NetworkMessage{
		ChannelID: channelID,
		EventID:   sip.Sum(payload),
		OriginID:  n.me.NodeID(),
		TargetIDs: targetIDs,
		Payload:   payload,
	}

	return e, nil
}

// submit method submits the given event for the given channel to the overlay layer
// for processing; it is used by engines through conduits.
// The Target needs to be added as a peer before submitting the message.
func (n *Network) submit(channelID uint8, event interface{}, targetIDs ...flow.Identifier) error {
	// genNetworkMessage the event to get payload and event ID
	message, err := n.genNetworkMessage(channelID, event, targetIDs...)
	if err != nil {
		return errors.Wrap(err, "could not cast the event into NetworkMessage")
	}
	// gossip

	// checks if the event is already in the cache
	ok := n.cache.Has(channelID, message.EventID)
	if ok {
		// returns nil and terminates sending the message since
		// the message already submitted to the network
		return nil
	}
	// storing event in the cache
	n.cache.Set(channelID, message.EventID, &message)

	// get all peers that haven't seen it yet
	//nodeIDs := n.top.Peers(peer.Not(peer.HasSeen(message.EventID))).NodeIDs()
	//if len(nodeIDs) == 0 {
	//	return nil
	//}
	err = n.send(message, targetIDs...)
	if err != nil {
		return errors.Wrap(err, "could not gossip event")
	}

	return nil
}

// send sends the message to the set of target ids through the middleware
// send is the last method within the pipeline of message shipping in network layer
// once it is called, the message slips through the network layer towards the middleware
func (n *Network) send(msg interface{}, nodeIDs ...flow.Identifier) error {
	// send the message through the peer connection
	var result *multierror.Error
	for _, nodeID := range nodeIDs {
		err := n.mw.Send(nodeID, msg)
		if err != nil {
			result = multierror.Append(result, err)
		}
	}

	return result.ErrorOrNil()
}
