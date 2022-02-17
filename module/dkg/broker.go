package dkg

import (
	"bytes"
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/rs/zerolog"
	"github.com/sethvargo/go-retry"

	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/model/fingerprint"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/messages"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/retrymiddleware"
)

// BrokerOpt is a functional option which modifies the DKG Broker config.
type BrokerOpt func(*BrokerConfig)

// BrokerConfig is configuration for the DKG Broker component.
type BrokerConfig struct {
	// PublishMaxRetries is the maximum number of times the broker will attempt
	// to broadcast a message or publish a result.
	PublishMaxRetries uint64
	// ReadMaxRetries is the max number of times the broker will attempt to
	// read messages before giving up.
	ReadMaxRetries uint64
	// RetryMaxConsecutiveFailures is the number of consecutive failures allowed
	// before we switch to a different Access client for subsequent attempts.
	RetryMaxConsecutiveFailures int
	// RetryInitialWait is the initial duration to wait between retries for all
	// retryable requests - increases exponentially for subsequent retries.
	RetryInitialWait time.Duration
	// RetryJitterPct is the percentage jitter to introduce to each retry interval
	// for all retryable requests.
	RetryJitterPct uint64
}

// DefaultBrokerConfig returns the default config for the DKG Broker component.
func DefaultBrokerConfig() BrokerConfig {
	return BrokerConfig{
		PublishMaxRetries:           10,
		ReadMaxRetries:              3,
		RetryMaxConsecutiveFailures: 2,
		RetryInitialWait:            time.Second,
		RetryJitterPct:              25,
	}
}

// Broker is an implementation of the DKGBroker interface which is intended to
// be used in conjunction with the DKG MessagingEngine for private messages, and
// with the DKG smart-contract for broadcast messages.
type Broker struct {
	config                    BrokerConfig
	log                       zerolog.Logger
	unit                      *engine.Unit
	dkgInstanceID             string                     // unique identifier of the current dkg run (prevent replay attacks)
	committee                 flow.IdentityList          // identities of DKG members
	me                        module.Local               // used for signing broadcast messages
	myIndex                   int                        // index of this instance in the committee
	dkgContractClients        []module.DKGContractClient // array of clients to communicate with the DKG smart contract in priority order for fallbacks during retries
	lastSuccessfulClientIndex int                        // index of the contract client that was last successful during retries
	tunnel                    *BrokerTunnel              // channels through which the broker communicates with the network engine
	privateMsgCh              chan messages.DKGMessage   // channel to forward incoming private messages to consumers
	broadcastMsgCh            chan messages.DKGMessage   // channel to forward incoming broadcast messages to consumers
	messageOffset             uint                       // offset for next broadcast messages to fetch
	shutdownCh                chan struct{}              // channel to stop the broker from listening

	broadcasts uint // broadcasts counts the number of attempted broadcasts

	clientLock    sync.Mutex // lock around updates to current client
	broadcastLock sync.Mutex // lock around outbound broadcasts
	pollLock      sync.Mutex // lock around polls to read inbound broadcasts
}

var _ module.DKGBroker = (*Broker)(nil)

// NewBroker instantiates a new epoch-specific broker capable of communicating
// with other nodes via a network engine and dkg smart-contract.
func NewBroker(
	log zerolog.Logger,
	dkgInstanceID string,
	committee flow.IdentityList,
	me module.Local,
	myIndex int,
	dkgContractClients []module.DKGContractClient,
	tunnel *BrokerTunnel,
	opts ...BrokerOpt,
) *Broker {

	config := DefaultBrokerConfig()
	for _, apply := range opts {
		apply(&config)
	}

	b := &Broker{
		config:             config,
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

		// NOTE: We're counting the number of times the underlying DKG requested
		// a broadcast so that we can detect an unhappy path (any time there is
		// more than 1 broadcast message per DKG) Thus incrementing broadcasts
		// before we perform the broadcasts is okay.
		b.broadcastLock.Lock()
		if b.broadcasts > 0 {
			// The warn-level log is used by the integration tests to check if this
			// func is called more than once within one epoch (unhappy path).
			b.log.Warn().Msgf("preparing to send DKG broadcast number %d with header %d", b.broadcasts+1, data[0])
		} else {
			b.log.Info().Msgf("preparing to send DKG message broadcast with header %d", data[0])
		}
		b.broadcasts++
		log := b.log.With().Uint("broadcast_number", b.broadcasts).Logger()
		b.broadcastLock.Unlock()

		bcastMsg, err := b.prepareBroadcastMessage(data)
		if err != nil {
			log.Fatal().Err(err).Msg("failed to create broadcast message")
		}

		backoff, err := retry.NewExponential(b.config.RetryInitialWait)
		if err != nil {
			log.Fatal().Err(err).Msg("create retry mechanism")
		}
		backoff = retry.WithMaxRetries(b.config.PublishMaxRetries, backoff)
		backoff = retry.WithJitterPercent(b.config.RetryJitterPct, backoff)

		clientIndex, dkgContractClient := b.getInitialContractClient()
		onMaxConsecutiveRetries := func(totalAttempts int) {
			clientIndex, dkgContractClient = b.updateContractClient(clientIndex)
			log.Warn().Msgf("broadcast: retrying on attempt (%d) with fallback access node at index (%d)", totalAttempts, clientIndex)
		}
		backoff = retrymiddleware.AfterConsecutiveFailures(b.config.RetryMaxConsecutiveFailures, backoff, onMaxConsecutiveRetries)

		b.broadcastLock.Lock()
		attempts := 1
		err = retry.Do(b.unit.Ctx(), backoff, func(ctx context.Context) error {
			err := dkgContractClient.Broadcast(bcastMsg)
			if err != nil {
				log.Error().Err(err).Msgf("error broadcasting, retrying (attempt %d)", attempts)
				attempts++
				return retry.RetryableError(err)
			}

			// update our last successful client index for future calls
			b.updateLastSuccessfulClient(clientIndex)
			return nil
		})
		b.broadcastLock.Unlock()

		// Various network conditions can result in errors while broadcasting DKG messages.
		// Because the overall DKG is resilient to individual message failures,
		// it is acceptable to log the error and move on.
		if err != nil {
			log.Error().Err(err).Msgf("failed to broadcast message after %d attempts", attempts)
			return
		}
		log.Info().Msgf("dkg broadcast successfully on attempt %d", attempts)
	})
}

// SubmitResult publishes the result of the DKG protocol to the smart contract.
func (b *Broker) SubmitResult(groupKey crypto.PublicKey, pubKeys []crypto.PublicKey) error {

	// If the DKG failed locally, we will get a nil key vector here. We need to convert
	// the nil slice to a slice of nil keys before submission.
	//
	// In general, if pubKeys does not have one key per participant, we cannot submit
	// a valid result - therefore we submit a nil vector (indicating that we have
	// completed the process, but we know that we don't have a valid result).
	if len(pubKeys) != len(b.committee) {
		b.log.Warn().Msgf("submitting dkg result with incomplete key vector (len=%d, expected=%d)", len(pubKeys), len(b.committee))
		// create a key vector with one nil entry for each committee member
		pubKeys = make([]crypto.PublicKey, len(b.committee))
	}

	backoff, err := retry.NewExponential(b.config.RetryInitialWait)
	if err != nil {
		b.log.Fatal().Err(err).Msg("failed to create retry mechanism")
	}
	backoff = retry.WithMaxRetries(b.config.PublishMaxRetries, backoff)
	backoff = retry.WithJitterPercent(b.config.RetryJitterPct, backoff)

	clientIndex, dkgContractClient := b.getInitialContractClient()
	onMaxConsecutiveRetries := func(totalAttempts int) {
		clientIndex, dkgContractClient = b.updateContractClient(clientIndex)
		b.log.Warn().Msgf("submit result: retrying on attempt (%d) with fallback access node at index (%d)", totalAttempts, clientIndex)
	}
	backoff = retrymiddleware.AfterConsecutiveFailures(b.config.RetryMaxConsecutiveFailures, backoff, onMaxConsecutiveRetries)

	attempts := 1
	err = retry.Do(b.unit.Ctx(), backoff, func(ctx context.Context) error {
		err := dkgContractClient.SubmitResult(groupKey, pubKeys)
		if err != nil {
			b.log.Error().Err(err).Msgf("error submitting DKG result, retrying (attempt %d)", attempts)
			attempts++
			return retry.RetryableError(err)
		}

		// update our last successful client index for future calls
		b.updateLastSuccessfulClient(clientIndex)
		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to submit dkg result after %d attempts: %w", attempts, err)
	}

	b.log.Info().Msgf("dkg result submitted successfully on attempt %d", attempts)
	return nil
}

// Disqualify flags that a node is misbehaving and got disqualified
func (b *Broker) Disqualify(node int, log string) {
	var nodeID flow.Identifier
	if node < len(b.committee) {
		nodeID = b.committee[node].NodeID
	}

	// The warn-level log is used by the integration tests to check if this method is called.
	b.log.Warn().Msgf("participant %d (this node) is disqualifying participant (index=%d, node_id=%s) because: %s",
		b.myIndex, node, nodeID, log)
}

// FlagMisbehavior warns that a node is misbehaving.
func (b *Broker) FlagMisbehavior(node int, log string) {
	var nodeID flow.Identifier
	if node < len(b.committee) {
		nodeID = b.committee[node].NodeID
	}

	// The warn-level log is used by the integration tests to check if this method is called.
	b.log.Warn().Msgf("participant %d (this node) is flagging participant (index=%d, node_id=%s) because: %s",
		b.myIndex, node, nodeID, log)
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

	backoff, err := retry.NewExponential(b.config.RetryInitialWait)
	if err != nil {
		b.log.Fatal().Err(err).Msg("failed to create retry mechanism")
	}
	backoff = retry.WithMaxRetries(b.config.ReadMaxRetries, backoff)
	backoff = retry.WithJitterPercent(b.config.RetryJitterPct, backoff)

	clientIndex, dkgContractClient := b.getInitialContractClient()
	onMaxConsecutiveRetries := func(totalAttempts int) {
		clientIndex, dkgContractClient = b.updateContractClient(clientIndex)
		b.log.Warn().Msgf("poll: retrying on attempt (%d) with fallback access node at index (%d)", totalAttempts, clientIndex)
	}
	backoff = retrymiddleware.AfterConsecutiveFailures(b.config.RetryMaxConsecutiveFailures, backoff, onMaxConsecutiveRetries)

	var msgs []messages.BroadcastDKGMessage
	attempt := 1
	err = retry.Do(b.unit.Ctx(), backoff, func(ctx context.Context) error {
		msgs, err = dkgContractClient.ReadBroadcast(b.messageOffset, referenceBlock)
		if err != nil {
			err = fmt.Errorf("could not read broadcast messages (attempt: %d, offset: %d, ref: %v): %w", attempt, b.messageOffset, referenceBlock, err)
			attempt++
			return retry.RetryableError(err)
		}

		// update our last successful client index for future calls
		b.updateLastSuccessfulClient(clientIndex)
		return nil
	})
	// Various network conditions can result in errors while reading DKG messages
	// We will read any missed messages during the next poll because messageOffset is not increased
	if err != nil {
		b.log.Error().Err(err).Msgf("failed to read messages after %d attempts", attempt)
		return nil
	}

	for _, msg := range msgs {
		ok, err := b.verifyBroadcastMessage(msg)
		if err != nil {
			b.log.Error().Err(err).Msg("unable to verify broadcast message")
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

// Shutdown stop the goroutine that listens to incoming private messages.
func (b *Broker) Shutdown() {
	close(b.shutdownCh)
}

/*~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~*/

// updateContractClient will return the last successful client index by default for all initial operations or else
// it will return the appropriate client index with respect to last successful and number of client.
func (b *Broker) updateContractClient(clientIndex int) (int, module.DKGContractClient) {
	b.clientLock.Lock()
	defer b.clientLock.Unlock()
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
	b.clientLock.Lock()
	defer b.clientLock.Unlock()
	return b.lastSuccessfulClientIndex, b.dkgContractClients[b.lastSuccessfulClientIndex]
}

// updateLastSuccessfulClient set lastSuccessfulClientIndex in concurrency safe way
func (b *Broker) updateLastSuccessfulClient(clientIndex int) {
	b.clientLock.Lock()
	defer b.clientLock.Unlock()

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
