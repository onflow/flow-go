package dkg

import (
	"bytes"
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/onflow/flow-go/engine"

	"github.com/sethvargo/go-retry"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/model/fingerprint"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/messages"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/retrymiddleware"
)

const (
	// retryMaxPublish is the maximum number of times the broker will attempt to broadcast
	// a message or publish a result
	retryMaxPublish = 8

	// retryMaxRead is the max number of times the broker will attempt to read messages
	retryMaxRead = 3

	// retryMaxConsecutiveFailures is the number of consecutive failures allowed before we attempt with fallback client
	retryMaxConsecutiveFailures = 2
	// retryDuration is the initial duration to wait between retries for all retryable
	// requests - increases exponentially for subsequent retries
	retryDuration = time.Second

	// retryJitterPercent is the percentage jitter to introduce to each retry interval
	// for all retryable requests
	retryJitterPercent = 25 // 25%
)

// Broker is an implementation of the DKGBroker interface which is intended to
// be used in conjunction with the DKG MessagingEngine for private messages, and
// with the DKG smart-contract for broadcast messages.
type Broker struct {
	log                       zerolog.Logger
	unit                      *engine.Unit
	dkgInstanceID             string                     // unique identifier of the current dkg run (prevent replay attacks)
	committee                 flow.IdentityList          // IDs of DKG members
	me                        module.Local               // used for signing bcast messages
	myIndex                   int                        // index of this instance in the committee
	dkgContractClients        []module.DKGContractClient // array of clients to communicate with the DKG smart contract in priority order for fallbacks during retries
	lastSuccessfulClientIndex int                        // index of the contract client that was last successful during retries
	tunnel                    *BrokerTunnel              // channels through which the broker communicates with the network engine
	privateMsgCh              chan messages.DKGMessage   // channel to forward incoming private messages to consumers
	broadcastMsgCh            chan messages.DKGMessage   // channel to forward incoming broadcast messages to consumers
	messageOffset             uint                       // offset for next broadcast messages to fetch
	shutdownCh                chan struct{}              // channel to stop the broker from listening

	broadcasts    uint       // broadcasts counts the number of successful broadcasts
	broadcastLock sync.Mutex // protects access to broadcasts count variable

	pollLock sync.Mutex // lock around polls to read inbound broadcasts
}

// NewBroker instantiates a new epoch-specific broker capable of communicating
// with other nodes via a network engine and dkg smart-contract.
func NewBroker(
	log zerolog.Logger,
	dkgInstanceID string,
	committee flow.IdentityList,
	me module.Local,
	myIndex int,
	dkgContractClients []module.DKGContractClient,
	tunnel *BrokerTunnel) *Broker {

	b := &Broker{
		log:                log.With().Str("component", "broker").Str("dkg_instance_id", dkgInstanceID).Logger(),
		unit:               engine.NewUnit(),
		dkgInstanceID:      dkgInstanceID,
		committee:          committee,
		me:                 me,
		myIndex:            myIndex,
		dkgContractClients: dkgContractClients,
		tunnel:             tunnel,
		privateMsgCh:       make(chan messages.DKGMessage),
		broadcastMsgCh:     make(chan messages.DKGMessage),
		shutdownCh:         make(chan struct{}),
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
	b.unit.Launch(func() {
		// NOTE: We're counting the number of times the underlying DKG
		// requested a broadcast so we can detect an unhappy path. Thus incrementing
		// broadcasts before we perform the broadcasts is okay.
		b.broadcastLock.Lock()
		if b.broadcasts > 0 {
			// The Warn log is used by the integration tests to check if this method
			// is called more than once within one epoch.
			b.log.Warn().Msgf("DKG broadcast number %d with header %d", b.broadcasts+1, data[0])
		} else {
			b.log.Info().Msgf("DKG message broadcast with header %d", data[0])
		}
		b.broadcasts++
		b.broadcastLock.Unlock()

		bcastMsg, err := b.prepareBroadcastMessage(data)
		if err != nil {
			b.log.Fatal().Err(err).Msg("failed to create broadcast message")
		}

		expRetry, err := retry.NewExponential(retryDuration)
		if err != nil {
			b.log.Fatal().Err(err).Msg("create retry mechanism")
		}
		maxedExpRetry := retry.WithMaxRetries(retryMaxPublish, expRetry)
		withJitter := retry.WithJitterPercent(retryJitterPercent, maxedExpRetry)

		clientIndex, dkgContractClient := b.getInitialContractClient()
		onMaxConsecutiveRetries := func(totalAttempts int) {
			clientIndex, dkgContractClient = b.updateContractClient(clientIndex)
			b.log.Warn().Msgf("broadcast: retrying on attempt (%d) with fallback access node at index (%d)", totalAttempts, clientIndex)
		}
		afterConsecutiveFailures := retrymiddleware.AfterConsecutiveFailures(retryMaxConsecutiveFailures, withJitter, onMaxConsecutiveRetries)

		attempts := 1
		err = retry.Do(context.Background(), afterConsecutiveFailures, func(ctx context.Context) error {
			err := dkgContractClient.Broadcast(bcastMsg)
			if err != nil {
				b.log.Error().Err(err).Msgf("error broadcasting, retrying (%d)", attempts)
				attempts++
				return retry.RetryableError(err)
			}

			// update our last successful client index for future calls
			b.updateLastSuccessfulClient(clientIndex)
			return nil
		})

		// Various network can conditions can result in errors while broadcasting DKG messages,
		// because failure to send an individual DKG message doesn't necessarily result in local or global DKG failure
		// it is acceptable to log the error and move on.
		if err != nil {
			b.log.Error().Err(err).Msg("failed to broadcast message")
		}
	})
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
	// We only issue one poll at a time to avoid delivering duplicate broadcast messages.
	// The messageOffset determines which messages are retrieved by a Poll,
	// and is not updated until the end of this function.
	b.pollLock.Lock()
	defer b.pollLock.Unlock()

	expRetry, err := retry.NewExponential(retryDuration)
	if err != nil {
		b.log.Fatal().Err(err).Msg("failed to create retry mechanism")
	}
	maxedExpRetry := retry.WithMaxRetries(retryMaxRead, expRetry)
	withJitter := retry.WithJitterPercent(retryJitterPercent, maxedExpRetry)

	clientIndex, dkgContractClient := b.getInitialContractClient()
	onMaxConsecutiveRetries := func(totalAttempts int) {
		clientIndex, dkgContractClient = b.updateContractClient(clientIndex)
		b.log.Warn().Msgf("poll: retrying on attempt (%d) with fallback access node at index (%d)", totalAttempts, clientIndex)
	}
	afterConsecutiveFailures := retrymiddleware.AfterConsecutiveFailures(retryMaxConsecutiveFailures, withJitter, onMaxConsecutiveRetries)

	var msgs []messages.BroadcastDKGMessage
	err = retry.Do(b.unit.Ctx(), afterConsecutiveFailures, func(ctx context.Context) error {
		msgs, err = dkgContractClient.ReadBroadcast(b.messageOffset, referenceBlock)
		if err != nil {
			err = fmt.Errorf("could not read broadcast messages(offset: %d, ref: %v): %w", b.messageOffset, referenceBlock, err)
			return retry.RetryableError(err)
		}

		// update our last successful client index for future calls
		b.updateLastSuccessfulClient(clientIndex)
		return nil
	})
	// Various network conditions can result in errors while reading DKG messages
	// We will read any missed messages during the next poll because messageOffset is not increased
	if err != nil {
		b.log.Error().Err(err).Msg("failed to read messages")
		return nil
	}

	b.unit.Lock()
	defer b.unit.Unlock()
	for _, msg := range msgs {
		ok, err := b.verifyBroadcastMessage(msg)
		if err != nil {
			b.log.Error().Err(err).Msg("bad broadcast message")
			continue
		}
		if !ok {
			b.log.Error().Msg("invalid signature on broadcast dkg message")
			continue
		}
		b.log.Debug().Msgf("forwarding broadcast message to controller")
		b.broadcastMsgCh <- msg.DKGMessage
	}

	// update message offset to use for future polls, this avoids forwarding the
	// same message more than once
	b.messageOffset += uint(len(msgs))
	return nil
}

// SubmitResult publishes the result of the DKG protocol to the smart contract.
func (b *Broker) SubmitResult(pubKey crypto.PublicKey, groupKeys []crypto.PublicKey) error {
	expRetry, err := retry.NewExponential(retryDuration)
	if err != nil {
		b.log.Fatal().Err(err).Msg("failed to create retry mechanism")
	}
	maxedExpRetry := retry.WithMaxRetries(retryMaxPublish, expRetry)
	withJitter := retry.WithJitterPercent(retryJitterPercent, maxedExpRetry)

	clientIndex, dkgContractClient := b.getInitialContractClient()
	onMaxConsecutiveRetries := func(totalAttempts int) {
		clientIndex, dkgContractClient = b.updateContractClient(clientIndex)
		b.log.Warn().Msgf("submit result: retrying on attempt (%d) with fallback access node at index (%d)", totalAttempts, clientIndex)
	}
	afterConsecutiveFailures := retrymiddleware.AfterConsecutiveFailures(retryMaxConsecutiveFailures, withJitter, onMaxConsecutiveRetries)
	err = retry.Do(context.Background(), afterConsecutiveFailures, func(ctx context.Context) error {
		err := dkgContractClient.SubmitResult(pubKey, groupKeys)
		if err != nil {
			b.log.Error().Err(err).Msg("error submitting DKG result, retrying")
			return retry.RetryableError(err)
		}

		// update our last successful client index for future calls
		b.updateLastSuccessfulClient(clientIndex)
		return nil
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

// updateContractClient will return the last successful client index by default for all initial operations or else
// it will return the appropriate client index with respect to last successful and number of client.
func (b *Broker) updateContractClient(clientIndex int) (int, module.DKGContractClient) {
	b.unit.Lock()
	defer b.unit.Unlock()
	if clientIndex == b.lastSuccessfulClientIndex {
		if clientIndex == len(b.dkgContractClients)-1 {
			clientIndex = 0
		} else {
			clientIndex++
		}
	} else {
		clientIndex = b.lastSuccessfulClientIndex
	}

	return clientIndex, b.dkgContractClients[clientIndex]
}

// getInitialContractClient will return the last successful contract client or the initial
func (b *Broker) getInitialContractClient() (int, module.DKGContractClient) {
	b.unit.Lock()
	defer b.unit.Unlock()
	return b.lastSuccessfulClientIndex, b.dkgContractClients[b.lastSuccessfulClientIndex]
}

// updateLastSuccessfulClient set lastSuccessfulClientIndex in concurrency safe way
func (b *Broker) updateLastSuccessfulClient(clientIndex int) {
	b.unit.Lock()
	defer b.unit.Unlock()

	b.lastSuccessfulClientIndex = clientIndex
}

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
// Returns:
// * true, nil if the message contents are valid and have a valid signature
// * false, nil if the message contents are valid but have an invalid signature
// * false, err if the message contents are invalid, or could not be checked,
//   or the signature could not be checked
// TODO differentiate errors
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
