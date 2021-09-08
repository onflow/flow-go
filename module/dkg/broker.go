package dkg

import (
	"bytes"
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/sethvargo/go-retry"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/model/fingerprint"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/messages"
	"github.com/onflow/flow-go/module"
)

// retryMax is the maximum number of times the broker will attempt to broadcast
// a message or publish a result
const retryMax = 8

// retryMilliseconds is the number of milliseconds to wait between retries
const retryMilliseconds = 1000 * time.Millisecond

// Broker is an implementation of the DKGBroker interface which is intended to
// be used in conjuction with the DKG MessagingEngine for private messages, and
// with the DKG smart-contract for broadcast messages.
type Broker struct {
	sync.Mutex
	log               zerolog.Logger
	dkgInstanceID     string                   // unique identifier of the current dkg run (prevent replay attacks)
	committee         flow.IdentityList        // IDs of DKG members
	me                module.Local             // used for signing bcast messages
	myIndex           int                      // index of this instance in the committee
	dkgContractClient module.DKGContractClient // client to communicate with the DKG smart contract
	tunnel            *BrokerTunnel            // channels through which the broker communicates with the network engine
	privateMsgCh      chan messages.DKGMessage // channel to forward incoming private messages to consumers
	broadcastMsgCh    chan messages.DKGMessage // channel to forward incoming broadcast messages to consumers
	messageOffset     uint                     // offset for next broadcast messages to fetch
	shutdownCh        chan struct{}            // channel to stop the broker from listening
	broadcasts        uint                     // broadcasts counts the number of successful broadcasts
}

// NewBroker instantiates a new epoch-specific broker capable of communicating
// with other nodes via a network engine and dkg smart-contract.
func NewBroker(
	log zerolog.Logger,
	dkgInstanceID string,
	committee flow.IdentityList,
	me module.Local,
	myIndex int,
	dkgContractClient module.DKGContractClient,
	tunnel *BrokerTunnel) *Broker {

	b := &Broker{
		log:               log.With().Str("component", "broker").Str("dkg_instance_id", dkgInstanceID).Logger(),
		dkgInstanceID:     dkgInstanceID,
		committee:         committee,
		me:                me,
		myIndex:           myIndex,
		dkgContractClient: dkgContractClient,
		tunnel:            tunnel,
		privateMsgCh:      make(chan messages.DKGMessage),
		broadcastMsgCh:    make(chan messages.DKGMessage),
		shutdownCh:        make(chan struct{}),
	}

	go b.listen()

	return b
}

/*~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~
Implement DKGBroker
~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~*/

// GetIndex returns the index of this node in the committee list.
func (b *Broker) GetIndex() int {
	return b.myIndex
}

// PrivateSend sends a DKGMessage to a destination over a private channel. It
// appends the current DKG instance ID to the message.
func (b *Broker) PrivateSend(dest int, data []byte) {
	if dest >= len(b.committee) || dest < 0 {
		b.log.Error().Msgf("destination id out of range: %d", dest)
		return
	}
	dkgMessageOut := messages.PrivDKGMessageOut{
		DKGMessage: messages.NewDKGMessage(b.myIndex, data, b.dkgInstanceID),
		DestID:     b.committee[dest].NodeID,
	}
	b.tunnel.SendOut(dkgMessageOut)
}

// Broadcast signs and broadcasts a message to all participants.
func (b *Broker) Broadcast(data []byte) {
	if b.broadcasts > 0 {
		// The Warn log is used by the integration tests to check if this method
		// is called more than once within one epoch.
		b.log.Warn().Msgf("DKG broadcast number %d with header %d", b.broadcasts+1, data[0])
	} else {
		b.log.Info().Msgf("DKG message broadcast with header %d", data[0])
	}
	bcastMsg, err := b.prepareBroadcastMessage(data)
	if err != nil {
		b.log.Fatal().Err(err).Msg("failed to create broadcast message")
	}

	expRetry, err := retry.NewExponential(retryMilliseconds)
	if err != nil {
		b.log.Fatal().Err(err).Msg("create retry mechanism")
	}
	maxedExpRetry := retry.WithMaxRetries(retryMax, expRetry)

	go func() {
		err = retry.Do(context.Background(), maxedExpRetry, func(ctx context.Context) error {
			err := b.dkgContractClient.Broadcast(bcastMsg)
			if err != nil {
				b.log.Error().Err(err).Msg("error broadcasting DKG result, retrying")
			}
			return retry.RetryableError(err)
		})

		if err != nil {
			b.log.Fatal().Err(err).Msg("failed to broadcast message")
		}
		b.broadcasts++
	}()
}

// Disqualify flags that a node is misbehaving and got disqualified
func (b *Broker) Disqualify(node int, log string) {
	// The Warn log is used by the integration tests to check if this method is
	// called.
	b.log.Warn().Msgf("participant %d is disqualifying participant %d because: %s", b.myIndex, node, log)
}

// FlagMisbehavior warns that a node is misbehaving.
func (b *Broker) FlagMisbehavior(node int, log string) {
	// The Warn log is used by the integration tests to check if this method is
	// called.
	b.log.Warn().Msgf("participant %d is flagging participant %d because: %s", b.myIndex, node, log)
}

// GetPrivateMsgCh returns the channel through which consumers can receive
// incoming private DKG messages.
func (b *Broker) GetPrivateMsgCh() <-chan messages.DKGMessage {
	return b.privateMsgCh
}

// GetBroadcastMsgCh returns the channel through which consumers can receive
// incoming broadcast DKG messages.
func (b *Broker) GetBroadcastMsgCh() <-chan messages.DKGMessage {
	return b.broadcastMsgCh
}

// Poll calls the DKG smart contract to get missing DKG messages for the current
// epoch, and forwards them to the msgCh. It should be called with the ID of a
// block whose seal is finalized. The function doesn't return until the received
// messages are processed by the consumer because b.msgCh is not buffered.
func (b *Broker) Poll(referenceBlock flow.Identifier) error {
	b.Lock()
	defer b.Unlock()
	msgs, err := b.dkgContractClient.ReadBroadcast(b.messageOffset, referenceBlock)
	if err != nil {
		return fmt.Errorf("could not read broadcast messages(offset: %d, ref: %v): %w", b.messageOffset, referenceBlock, err)
	}
	for _, msg := range msgs {
		ok, err := b.verifyBroadcastMessage(msg)
		if err != nil {
			b.log.Error().Err(err).Msg("bad broadcast message")
			continue
		}
		if !ok {
			b.log.Debug().Msg("invalid signature on broadcast dkg message")
			continue
		}
		b.log.Debug().Msgf("forwarding broadcast message to controller")
		b.broadcastMsgCh <- msg.DKGMessage
	}
	b.messageOffset += uint(len(msgs))
	return nil
}

// SubmitResult publishes the result of the DKG protocol to the smart contract.
func (b *Broker) SubmitResult(pubKey crypto.PublicKey, groupKeys []crypto.PublicKey) error {
	expRetry, err := retry.NewExponential(retryMilliseconds)
	if err != nil {
		b.log.Fatal().Err(err).Msg("failed to create retry mechanism")
	}
	maxedExpRetry := retry.WithMaxRetries(retryMax, expRetry)

	err = retry.Do(context.Background(), maxedExpRetry, func(ctx context.Context) error {
		err := b.dkgContractClient.SubmitResult(pubKey, groupKeys)
		if err != nil {
			b.log.Error().Err(err).Msg("error submitting DKG result, retrying")
		}
		return retry.RetryableError(err)
	})

	if err != nil {
		b.log.Fatal().Err(err).Msg("failed to submit dkg result")
	}

	b.log.Debug().Msg("dkg result submitted")
	return nil
}

// Shutdown stop the goroutine that listens to incoming private messages.
func (b *Broker) Shutdown() {
	close(b.shutdownCh)
}

/*~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~*/

// listen is a blocking call that processes incoming messages from the network
// engine.
func (b *Broker) listen() {
	for {
		select {
		case msg := <-b.tunnel.MsgChIn:
			b.onPrivateMessage(msg.OriginID, msg.DKGMessage)
		case <-b.shutdownCh:
			return
		}
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
	nodeID := b.committee[msg.Orig].NodeID
	if !bytes.Equal(nodeID[:], originID[:]) {
		b.log.Error().Msgf("bad message: OriginID (%v) does not match committee member %d (%v)", originID, msg.Orig, nodeID)
		return
	}
	b.privateMsgCh <- msg
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
	if msg.Orig >= uint64(len(b.committee)) {
		return fmt.Errorf("origin id out of range: %d", msg.Orig)
	}
	return nil
}

// prepareBroadcastMessage creates BroadcastDKGMessage with a signature from the
// node's staking key.
func (b *Broker) prepareBroadcastMessage(data []byte) (messages.BroadcastDKGMessage, error) {
	dkgMessage := messages.NewDKGMessage(
		b.myIndex,
		data,
		b.dkgInstanceID,
	)
	sigData := fingerprint.Fingerprint(dkgMessage)
	signature, err := b.me.Sign(sigData[:], NewDKGMessageHasher())
	if err != nil {
		return messages.BroadcastDKGMessage{}, err
	}
	bcastMsg := messages.BroadcastDKGMessage{
		DKGMessage: dkgMessage,
		Signature:  signature,
	}
	return bcastMsg, nil
}

// verifyBroadcastMessage checks the DKG instance and Origin of a broadcast
// message, as well as the signature against the staking key of the sender.
func (b *Broker) verifyBroadcastMessage(bcastMsg messages.BroadcastDKGMessage) (bool, error) {
	err := b.checkMessageInstanceAndOrigin(bcastMsg.DKGMessage)
	if err != nil {
		return false, err
	}
	origin := b.committee[bcastMsg.Orig]
	signData := fingerprint.Fingerprint(bcastMsg.DKGMessage)
	return origin.StakingPubKey.Verify(
		bcastMsg.Signature,
		signData[:],
		NewDKGMessageHasher(),
	)
}
