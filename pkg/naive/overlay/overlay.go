// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package overlay

import (
	"fmt"
	"math/rand"
	"time"

	"github.com/pkg/errors"
	"github.com/rs/zerolog"

	"github.com/dapperlabs/flow-go/pkg/codec"
	"github.com/dapperlabs/flow-go/pkg/model/message"
	"github.com/dapperlabs/flow-go/pkg/naive"
)

// Overlay represents the overlay network of our peer-to-peer network, including
// the protocols for handshakes, authentication, gossiping and heartbeats.
type Overlay struct {
	log     zerolog.Logger
	mw      Middleware
	codec   codec.Codec
	state   State
	cache   Cache
	engines map[uint8]naive.Engine
	id      string
}

// New creates a new protocol layer instance, using the given middleware to
// communicate to direct peers, using the given codec for serialization, and
// using the given state & cache interfaces to track volatile information.
func New(log zerolog.Logger, mw Middleware, codec codec.Codec, state State, cache Cache) (*Overlay, error) {
	rand.Seed(time.Now().UnixNano())
	o := &Overlay{
		log:     log,
		mw:      mw,
		codec:   codec,
		state:   state,
		cache:   cache,
		engines: make(map[uint8]naive.Engine),
		id:      fmt.Sprint(rand.Uint64()),
	}
	mw.GetAddress(o.HandleAddress)
	mw.RunHandshake(o.HandleHandshake)
	mw.RunCleanup(o.HandleCleanup)
	mw.OnReceive(o.HandleReceive)
	return o, nil
}

// Register will register the given engine with the given unique engine code,
// returning a conduit to directly submit messages to the message bus of the
// engine.
func (o *Overlay) Register(code uint8, engine naive.Engine) (naive.Conduit, error) {

	// check if the engine code is already taken
	_, ok := o.engines[code]
	if ok {
		return nil, errors.Errorf("engine already registered (%d)", engine)
	}

	// create the conduit
	conduit := &Conduit{
		code: code,
		send: o.submit,
	}

	// register engine with provided code
	o.engines[code] = engine

	return conduit, nil
}

// HandleAddress implements a callback to provide the overlay layer with the
// addresses we want to establish new connections to.
func (o *Overlay) HandleAddress() (string, error) {
	return "", errors.New("not implemented")
}

// HandleHandshake implements a callback to validate new connections on the
// overlay layer, allowing us to implement peer authentication on first contact.
func (o *Overlay) HandleHandshake(conn Connection) (string, error) {

	// initialize our own authentication message
	out := &message.Auth{
		Node: o.id,
	}
	err := conn.Send(out)
	if err != nil {
		return "", errors.Wrap(err, "could not send outgoing auth")
	}

	// read their authentication message
	msg, err := conn.Receive()
	if err != nil {
		return "", errors.Wrap(err, "could not receive incoming auth")
	}

	// type assert the response
	in, ok := msg.(*message.Auth)
	if !ok {
		return "", errors.Errorf("invalid type for incoming auth (%T)", msg)
	}

	// check if we are already connected to this peer
	ok = o.state.Alive(in.Node)
	if ok {
		return "", errors.Errorf("node already connected (%s)", in.Node)
	}

	// TODO: do validation of the remote node ID

	// initialize the peer state using the address as its ID
	o.state.Init(in.Node)

	return in.Node, nil
}

// HandleCleanup implements a callback to handle peers that have been dropped
// by the middleware layer.
func (o *Overlay) HandleCleanup(node string) error {

	// drop the peer state using the ID we registered
	o.state.Drop(node)

	return nil
}

// HandleMessage provides a callback to handle the incoming message from the
// given peer.
func (o *Overlay) HandleReceive(peer string, msg interface{}) error {

	// NOTE: this is a good illustration of the question about how to add context
	// to the logging; here, we would like to add the peer ID, and ideally also
	// the message type, so we see it whenever it is logged

	var err error
	switch m := msg.(type) {
	case *message.Announce:
		err = o.processAnnounce(peer, m)
	case *message.Request:
		err = o.processRequest(peer, m)
	case *message.Event:
		err = o.processEvent(peer, m)
	default:
		err = errors.Errorf("invalid message type (%T)", msg)
	}
	if err != nil {
		return errors.Wrap(err, "could not process message")
	}

	return nil
}

// submit will submit the given event for the given engine to the overlay
// network, providing a list of recipients that should receive it.
func (o *Overlay) submit(code uint8, event interface{}, recipients ...string) error {

	// check if engine is registered
	engine, ok := o.engines[code]
	if !ok {
		return errors.Errorf("could not find engine (%d)", code)
	}

	// get the identifier of the event
	id, err := engine.Identify(event)
	if err != nil {
		return errors.Wrap(err, "could not identify event")
	}

	// encode the payload of the event
	payload, err := o.codec.Encode(event)
	if err != nil {
		return errors.Wrap(err, "could not encode event")
	}

	// cache the encoded payload for this event
	o.cache.Add(id, payload)

	// announce the event to the network
	ann := &message.Announce{
		Engine: code,
		ID:     id,
	}
	err = o.announce(ann)
	if err != nil {
		return errors.Wrap(err, "could not announce event")
	}

	return nil
}

// processAnnounce will process an announce message from the network and bubble
// the event up to the relevant engine if it is not already in our cache (which
// would indicate that it was already processed recently).
func (o *Overlay) processAnnounce(peer string, ann *message.Announce) error {

	// mark the entity as seen for this node, so we don't announce/send it to
	// them; if we have already cached it, skip further processing
	o.state.Seen(peer, ann.ID)
	ok := o.cache.Has(ann.ID)
	if !ok {
		return nil
	}

	// at this point, the event is probably unknown to our engines, so request the
	// full event payload
	req := &message.Request{
		Engine: ann.Engine,
		ID:     ann.ID,
	}
	err := o.mw.Send(peer, req)
	if err != nil {
		return errors.Wrap(err, "could not send request")
	}

	return nil
}

// processRequest will process a request for an event from a peer and send him
// the full payload if we find it in our cache or from the respective engine.
func (o *Overlay) processRequest(peer string, req *message.Request) error {

	// for stuff in our cache, it doesn't matter if the engine is running, as we
	// could just be serving as a hop, so just sen the cached payload
	payload, ok := o.cache.Get(req.ID)
	if ok {
		return o.reply(peer, req.Engine, payload)
	}

	// if it's not cached, check if we are running the given engine.
	engine, ok := o.engines[req.Engine]
	if !ok {
		return errors.Errorf("could not find engine (%d)", req.Engine)
	}

	// retrieve the event from the engine
	event, err := engine.Retrieve(req.ID)
	if err != nil {
		return errors.Wrap(err, "could not request event")
	}

	// encode the event and cache it
	payload, err = o.codec.Encode(event)
	if err != nil {
		return errors.Wrap(err, "could not encode event")
	}
	o.cache.Add(req.ID, payload)

	// send the reply
	return o.reply(peer, req.Engine, payload)
}

// processEvent will process the payload of an event sent to us by a peer.
func (o *Overlay) processEvent(peer string, msg *message.Event) error {

	// TODO: we can't cache it here without knowing the ID, so maybe we should
	// embed the ID in the message, or make message identification an engine
	// available on all peers

	// check if engine is registered
	engine, ok := o.engines[msg.Engine]
	if !ok {
		return errors.Errorf("could not find engine (%d)", msg.Engine)
	}

	// decode the event
	event, err := o.codec.Decode(msg.Payload)
	if err != nil {
		return errors.Wrap(err, "could not decode event")
	}

	// get the event id
	id, err := engine.Identify(event)
	if err != nil {
		return errors.Wrap(err, "could not identify event")
	}

	// check if payload is already in cache
	ok = o.cache.Has(id)
	if ok {
		return nil
	}
	o.cache.Add(id, msg.Payload)

	// check if we have a handler registered for this engine
	err = engine.Receive(msg.Origin, event)
	if err != nil {
		return errors.Wrap(err, "could not process payload")
	}

	// broadcast announcement to other peers
	ann := &message.Announce{
		Engine: msg.Engine,
		ID:     id,
	}
	err = o.announce(ann)
	if err != nil {
		return errors.Wrap(err, "could not announce event")
	}

	return nil
}

// announce will announce the entity to the network for all those peers that
// have not seen it yet.
func (o *Overlay) announce(ann *message.Announce) error {

	// create the announce message
	peers := o.state.Peers(HasNotSeen(ann.ID)).IDs()
	for _, id := range peers {
		err := o.mw.Send(id, ann)
		if err != nil {
			return errors.Wrapf(err, "could not send announcement (%v)", id)
		}
	}

	return nil
}

func (o *Overlay) reply(peer string, engine uint8, payload []byte) error {

	// create and send the event
	event := &message.Event{
		Engine:  engine,
		Origin:  o.id,
		Payload: payload,
	}
	err := o.mw.Send(peer, event)
	if err != nil {
		return errors.Wrap(err, "could not send event")
	}

	return nil
}
