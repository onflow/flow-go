package dkg

import (
	"bytes"
	"fmt"
	"sync"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/messages"
	"github.com/onflow/flow-go/module"
)

// Broker is an implementation of the DKGBroker interface which is intended to
// be used in conjuction with the DKG MessagingEngine for private messages, and
// with the DKG smart-contract for broadcast messages.
type Broker struct {
	sync.Mutex
	log               zerolog.Logger
	dkgInstanceID     string                   // unique identifier of the current dkg run (prevent replay attacks)
	committee         flow.IdentifierList      // IDs of DKG members
	myIndex           int                      // index of this instance in the committee
	dkgContractClient module.DKGContractClient // client to communicate with the DKG smart contract
	tunnel            *BrokerTunnel            // channels through which the broker communicates with the network engine
	msgCh             chan messages.DKGMessage // channel to forward incoming messages to consumers
	messageOffset     uint                     // offset for next broadcast messages to fetch
}

// NewBroker instantiates a new epoch-specific broker capable of communicating
// with other nodes via a network engine and dkg smart-contract.
func NewBroker(
	log zerolog.Logger,
	dkgInstanceID string,
	committee flow.IdentifierList,
	myIndex int,
	dkgContractClient module.DKGContractClient,
	tunnel *BrokerTunnel) *Broker {

	b := &Broker{
		log:               log.With().Str("component", "broker").Str("dkg_instance_id", dkgInstanceID).Logger(),
		dkgInstanceID:     dkgInstanceID,
		committee:         committee,
		myIndex:           myIndex,
		dkgContractClient: dkgContractClient,
		tunnel:            tunnel,
		msgCh:             make(chan messages.DKGMessage),
	}

	go b.listen()

	return b
}

/*~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~
Implement DKGBroker
~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~*/

// PrivateSend sends a DKGMessage to a destination over a private channel. It
// appends the current DKG instance ID to the message.
func (b *Broker) PrivateSend(dest int, data []byte) {
	if dest >= len(b.committee) || dest < 0 {
		b.log.Error().Msgf("destination id out of range: %d", dest)
		return
	}
	dkgMessageOut := messages.DKGMessageOut{
		DKGMessage: messages.NewDKGMessage(b.myIndex, data, b.dkgInstanceID),
		DestID:     b.committee[dest],
	}
	b.tunnel.SendOut(dkgMessageOut)
}

// Broadcast broadcasts a message to all participants.
func (b *Broker) Broadcast(data []byte) {
	dkgMessage := messages.NewDKGMessage(
		b.myIndex,
		data,
		b.dkgInstanceID,
	)
	err := b.dkgContractClient.Broadcast(dkgMessage)
	if err != nil {
		b.log.Error().Err(err).Msg("could not broadcast message")
	}
}

// Disqualify flags that a node is misbehaving and got disqualified
func (b *Broker) Disqualify(node int, log string) {
	b.log.Warn().Msgf("participant %d is disqualifying participant %d because: %s", b.myIndex, node, log)
}

// FlagMisbehavior warns that a node is misbehaving.
func (b *Broker) FlagMisbehavior(node int, log string) {
	b.log.Warn().Msgf("participant %d is flagging participant %d because: %s", b.myIndex, node, log)
}

// GetMsgCh returns the channel through which consumers can receive incoming
// DKG messages.
func (b *Broker) GetMsgCh() <-chan messages.DKGMessage {
	return b.msgCh
}

// Poll calls the DKG smart contract to get missing DKG messages for the current
// epoch, and forwards them to the msgCh. It should be called with the ID of
// block whose seal is finalized.
func (b *Broker) Poll(referenceBlock flow.Identifier) error {
	b.Lock()
	defer b.Unlock()
	msgs, err := b.dkgContractClient.ReadBroadcast(b.messageOffset, referenceBlock)
	if err != nil {
		return fmt.Errorf("could not read broadcast messages: %w", err)
	}
	for _, msg := range msgs {
		err := b.checkMessageInstanceAndOrigin(msg)
		if err != nil {
			b.log.Error().Err(err).Msg("bad broadcast message")
			continue
		}
		b.log.Debug().Msgf("forwarding broadcast message to controller")
		b.msgCh <- msg
	}
	b.messageOffset += uint(len(msgs))
	return nil
}

// SubmitResult publishes the result of the DKG protocol to the smart contract.
func (b *Broker) SubmitResult(res []crypto.PublicKey) error {
	return b.dkgContractClient.SubmitResult(res)
}

/*~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~*/

// listen is a blocking call that processes incoming messages from the network
// engine.
func (b *Broker) listen() {
	for {
		msg := <-b.tunnel.MsgChIn
		b.onPrivateMessage(msg.OriginID, msg.DKGMessage)
	}
}

// onPrivateMessage verifies the integrity of an incoming message and forwards
// it to consumers via the msgCh.
func (b *Broker) onPrivateMessage(originID flow.Identifier, msg messages.DKGMessage) {
	err := b.checkMessageInstanceAndOrigin(msg)
	if err != nil {
		b.log.Err(err).Msg("bad message")
		return
	}
	// check that the message's origin matches the sender's flow identifier
	nodeID := b.committee[msg.Orig]
	if !bytes.Equal(nodeID[:], originID[:]) {
		b.log.Error().Msgf("bad message: OriginID (%v) does not match committee member %d (%v)", originID, msg.Orig, nodeID)
		return
	}
	b.msgCh <- msg
}

// checkMessageInstanceAndOrigin returns an error if the message's dkgInstanceID
// does not correspond to the current instance, or if the message's origin index
// is out of range with respect to the committee list.
func (b *Broker) checkMessageInstanceAndOrigin(msg messages.DKGMessage) error {
	// check that the message corresponds to the current epoch
	if b.dkgInstanceID != msg.DKGInstanceID {
		return fmt.Errorf("wrong DKG instance. Got %s, want %s", msg.DKGInstanceID, b.dkgInstanceID)
	}
	// check that the message's origin is not out of range
	if msg.Orig >= len(b.committee) || msg.Orig < 0 {
		return fmt.Errorf("origin id out of range: %d", msg.Orig)
	}
	return nil
}
