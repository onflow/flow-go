package processor

import (
	"bytes"
	"fmt"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/messages"
	msg "github.com/onflow/flow-go/model/messages"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/network"
)

// Engine implements the crypto DKGProcessor interface which provides means for
// DKG nodes to exchange private and public messages. The same instance can be
// used across epochs, but transitions should be accompanied by a call to
// SetEpoch, such that messages can be prepended with the appropriate epoch
// ID.
type Engine struct {
	unit              *engine.Unit
	log               zerolog.Logger
	me                module.Local
	conduit           network.Conduit          // network conduit for sending and receiving private messages
	dkgContractClient module.DKGContractClient // client to communicate with the DKG smart contract
	msgCh             chan msg.DKGMessage      // channel to forward DKG messages to the consumer
	committee         flow.IdentifierList      // IDs of DKG members
	myIndex           int                      // index of this instance in the committee
	dkgInstanceID     string                   // unique identifier of the current dkg run (prevent replay attacks)
	messageOffset     uint                     // index of next broadcast message to query
}

// New returns a new DKGProcessor engine. Instances participating in a common
// DKG protocol must use the same _ordered_ list of node identifiers, because
// senders and recipients are identified by their index in this list.
// The msgCh is used to forward incoming DKG messages to the consumer, such as
// the Controller implemented in the DKG module.
func New(
	logger zerolog.Logger,
	net module.Network,
	me module.Local,
	msgCh chan msg.DKGMessage,
	committee flow.IdentifierList,
	dkgContractClient module.DKGContractClient,
	dkgInstanceID string) (*Engine, error) {

	log := logger.With().Str("engine", "dkg-processor").Logger()

	index := -1
	for i, id := range committee {
		if id == me.NodeID() {
			index = i
			break
		}
	}
	if index < 0 {
		return nil, fmt.Errorf("dkg-processor engine id does not belong to dkg committee")
	}

	eng := Engine{
		unit:              engine.NewUnit(),
		log:               log,
		me:                me,
		msgCh:             msgCh,
		committee:         committee,
		myIndex:           index,
		dkgInstanceID:     dkgInstanceID,
		dkgContractClient: dkgContractClient,
	}

	var err error
	eng.conduit, err = net.Register(engine.DKGCommittee, &eng)
	if err != nil {
		return nil, fmt.Errorf("could not register dkg-processor engine: %w", err)
	}

	return &eng, nil
}

// Ready implements the module ReadyDoneAware interface. It returns a channel
// that will close when the engine has successfully
// started.
//
// It instructs the engine to periodically read broadcast messages from the DKG
// smart contract.
func (e *Engine) Ready() <-chan struct{} {
	return e.unit.Ready()
}

// Done implements the module ReadyDoneAware interface. It returns a channel
// that will close when the engine has successfully stopped.
func (e *Engine) Done() <-chan struct{} {
	return e.unit.Done()
}

// SubmitLocal implements the network Engine interface
func (e *Engine) SubmitLocal(event interface{}) {
	e.Submit(e.me.NodeID(), event)
}

// Submit implements the network Engine interface
func (e *Engine) Submit(originID flow.Identifier, event interface{}) {
	e.unit.Launch(func() {
		err := e.Process(originID, event)
		if err != nil {
			engine.LogError(e.log, err)
		}
	})
}

// ProcessLocal implements the network Engine interface
func (e *Engine) ProcessLocal(event interface{}) error {
	return e.Process(e.me.NodeID(), event)
}

// Process implements the network Engine interface
func (e *Engine) Process(originID flow.Identifier, event interface{}) error {
	return e.unit.Do(func() error {
		return e.process(originID, event)
	})
}

func (e *Engine) process(originID flow.Identifier, event interface{}) error {
	switch v := event.(type) {
	case msg.DKGMessage:
		return e.onPrivateMessage(originID, v)
	default:
		return fmt.Errorf("invalid event type (%T)", event)
	}
}

func (e *Engine) onPrivateMessage(originID flow.Identifier, msg msg.DKGMessage) error {
	err := e.checkMessageInstanceAndOrigin(msg)
	if err != nil {
		return err
	}
	// check that the message's origin matches the sender's flow identifier
	nodeID := e.committee[msg.Orig]
	if !bytes.Equal(nodeID[:], originID[:]) {
		return fmt.Errorf("OriginID (%v) does not match committee member %d (%v)", originID, msg.Orig, nodeID)
	}
	e.log.Debug().Msgf("forwarding private message to controller")
	e.msgCh <- msg
	return nil
}

// fetchBroadcastMessages calls the DKG smart contract to get missing DKG
// messages for the current epoch, and forwards them to the msgCh.
// It should be called with the ID of block whose seal is finalized.
func (e *Engine) fetchBroadcastMessages(referenceBlock flow.Identifier) error {
	msgs, err := e.dkgContractClient.ReadBroadcast(e.messageOffset, referenceBlock)
	if err != nil {
		return fmt.Errorf("could not read broadcast messages: %w", err)
	}
	for _, msg := range msgs {
		err := e.checkMessageInstanceAndOrigin(msg)
		if err != nil {
			e.log.Error().Err(err).Msg("bad broadcast message")
			continue
		}
		e.log.Debug().Msgf("forwarding broadcast message to controller")
		e.msgCh <- msg
	}
	e.messageOffset += uint(len(msgs))
	return nil
}

// checkMessageInstanceAndOrigin returns an error if the message's dkgInstanceID
// does not correspond to the current instance, or if the message's origin index
// is out of range with respect to the committee list.
func (e *Engine) checkMessageInstanceAndOrigin(msg messages.DKGMessage) error {
	// check that the message corresponds to the current epoch
	if e.dkgInstanceID != msg.DKGInstanceID {
		return fmt.Errorf("wrong DKG instance. Got %s, want %s", msg.DKGInstanceID, e.dkgInstanceID)
	}
	// check that the message's origin is not out of range
	if msg.Orig >= len(e.committee) || msg.Orig < 0 {
		return fmt.Errorf("origin id out of range: %d", msg.Orig)
	}
	return nil
}

/*~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~
Implement DKGProcessor
~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~*/

// PrivateSend sends a DKGMessage to a destination over a private channel. It
// appends the current DKG instance ID to the message.
func (e *Engine) PrivateSend(dest int, data []byte) {
	if dest >= len(e.committee) || dest < 0 {
		e.log.Error().Msgf("destination id out of range: %d", dest)
		return
	}
	destID := e.committee[dest]
	dkgMessage := msg.NewDKGMessage(
		e.myIndex,
		data,
		e.dkgInstanceID,
	)
	err := e.conduit.Unicast(dkgMessage, destID)
	if err != nil {
		e.log.Error().
			Err(err).
			Str("dkg_instance_id", e.dkgInstanceID).
			Msgf("could not send private message to %v", destID)
	}
}

// Broadcast broadcasts a message to all participants.
func (e *Engine) Broadcast(data []byte) {
	dkgMessage := msg.NewDKGMessage(
		e.myIndex,
		data,
		e.dkgInstanceID,
	)
	err := e.dkgContractClient.Broadcast(dkgMessage)
	if err != nil {
		if err != nil {
			e.log.Error().
				Err(err).
				Str("dkg_instance_id", e.dkgInstanceID).
				Msg("could not broadcast message")
		}
	}
}

// Disqualify flags that a node is misbehaving and got disqualified
func (e *Engine) Disqualify(node int, log string) {}

// FlagMisbehavior warns that a node is misbehaving.
func (e *Engine) FlagMisbehavior(node int, log string) {}
