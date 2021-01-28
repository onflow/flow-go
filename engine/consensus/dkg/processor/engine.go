package processor

import (
	"bytes"
	"fmt"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/model/flow"
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
	conduit           network.Conduit
	dkgContractClient module.DKGContractClient
	msgCh             chan msg.DKGMessage
	committee         flow.IdentifierList
	myIndex           int
	epochCounter      uint64
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
	epochCounter uint64) (*Engine, error) {

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
		epochCounter:      epochCounter,
		dkgContractClient: dkgContractClient,
	}

	var err error
	eng.conduit, err = net.Register(engine.DKGCommittee, &eng)
	if err != nil {
		return nil, fmt.Errorf("could not register dkg-processor engine: %w", err)
	}

	return &eng, nil
}

// SetEpoch changes the epoch counter of the engine.
func (e *Engine) SetEpoch(epochCounter uint64) {
	e.unit.Lock()
	defer e.unit.Unlock()
	e.epochCounter = epochCounter
}

// Ready implements the module ReadyDoneAware interface. It returns a channel
// that will close when the engine has successfully
// started.
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
		return e.onMessage(originID, v)
	default:
		return fmt.Errorf("invalid event type (%T)", event)
	}
}

func (e *Engine) onMessage(originID flow.Identifier, msg msg.DKGMessage) error {

	// check that the message corresponds to the current epoch
	e.unit.Lock()
	defer e.unit.Unlock()
	if e.epochCounter != msg.EpochCounter {
		return fmt.Errorf("wrong epoch counter. Got %d, want %d", msg.EpochCounter, e.epochCounter)
	}

	// check that the message's origin is not out of range
	if msg.Orig >= len(e.committee) || msg.Orig < 0 {
		return fmt.Errorf("origin id out of range: %d", msg.Orig)
	}

	// check that the message's origin matches the sender's flow identifier
	nodeID := e.committee[msg.Orig]
	if !bytes.Equal(nodeID[:], originID[:]) {
		return fmt.Errorf("OriginID (%v) does not match committee member %d (%v)", originID, msg.Orig, nodeID)
	}

	e.log.Debug().Msgf("forwarding Message to controller")
	e.msgCh <- msg

	return nil
}

/*~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~
Implement DKGProcessor
~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~*/

// PrivateSend sends a DKGMessage to a destination over a private channel. It
// appends the current Epoch ID to the message.
func (e *Engine) PrivateSend(dest int, data []byte) {

	if dest >= len(e.committee) || dest < 0 {
		e.log.Error().Msgf("destination id out of range: %d", dest)
		return
	}

	destID := e.committee[dest]

	dkgMessage := msg.NewDKGMessage(
		e.myIndex,
		data,
		e.epochCounter,
	)

	err := e.conduit.Unicast(dkgMessage, destID)
	if err != nil {
		e.log.Error().Msgf("Could not send Message to %v: %v", destID, err)
		return
	}
}

// Broadcast broadcasts a message to all participants.
func (e *Engine) Broadcast(data []byte) {

	dkgMessage := msg.NewDKGMessage(
		e.myIndex,
		data,
		e.epochCounter,
	)

	err := e.dkgContractClient.Broadcast(dkgMessage)
	if err != nil {
		e.log.Error().Msgf("Could not broadcast message: %v", err)
	}
}

// Disqualify flags that a node is misbehaving and got disqualified
func (e *Engine) Disqualify(node int, log string) {}

// FlagMisbehavior warns that a node is misbehaving.
func (e *Engine) FlagMisbehavior(node int, log string) {}
