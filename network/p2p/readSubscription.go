package p2p

import (
	"context"
	"fmt"
	"strings"
	"sync"

	lcrypto "github.com/libp2p/go-libp2p-core/crypto"
	"github.com/libp2p/go-libp2p-core/peer"
	pubsub "github.com/libp2p/go-libp2p-pubsub"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/network/message"
	_ "github.com/onflow/flow-go/utils/binstat"
)

// readSubscription reads the messages coming in on the subscription and calls the given callback until
// the context of the subscription is cancelled. Additionally, it reports metrics
type readSubscription struct {
	ctx      context.Context
	log      zerolog.Logger
	sub      *pubsub.Subscription
	metrics  module.NetworkMetrics
	callback func(msg *message.Message, peerID peer.ID)
}

// newReadSubscription reads the messages coming in on the subscription
func newReadSubscription(ctx context.Context,
	sub *pubsub.Subscription,
	callback func(msg *message.Message, peerID peer.ID),
	log zerolog.Logger,
	metrics module.NetworkMetrics) *readSubscription {

	log = log.With().
		Str("channel", sub.Topic()).
		Logger()

	r := readSubscription{
		ctx:      ctx,
		log:      log,
		sub:      sub,
		callback: callback,
		metrics:  metrics,
	}

	return &r
}

// receiveLoop must be run in a goroutine. It continuously receives
// messages for the topic and calls the callback synchronously
func (r *readSubscription) receiveLoop(wg *sync.WaitGroup) {

	defer wg.Done()
	defer r.log.Debug().Msg("exiting receive routine")

	for {

		// read the next message from libp2p's subscription (blocking call)
		rawMsg, err := r.sub.Next(r.ctx)

		if err != nil {

			// middleware may have cancelled the context
			if err == context.Canceled {
				return
			}

			// subscription may have just been cancelled if node is being stopped or the topic has been unsubscribed,
			// don't log error in that case
			// (https://github.com/ipsn/go-ipfs/blob/master/gxlibs/github.com/libp2p/go-libp2p-pubsub/pubsub.go#L435)
			if strings.Contains(err.Error(), "subscription cancelled") {
				return
			}

			// log any other error
			r.log.Err(err).Msg("failed to read subscription message")

			return
		}

		pid, err := messageSigningID(rawMsg)
		if err != nil {
			r.log.Err(err).Msg("failed to validate peer ID of incoming message")
			return
		}

		var msg message.Message
		// convert the incoming raw message payload to Message type
		//bs := binstat.EnterTimeVal(binstat.BinNet+":wire>1protobuf2message", int64(len(rawMsg.Data)))
		err = msg.Unmarshal(rawMsg.Data)
		//binstat.Leave(bs)
		if err != nil {
			r.log.Err(err).Str("topic_message", msg.String()).Msg("failed to unmarshal message")
			return
		}

		// log metrics
		r.metrics.NetworkMessageReceived(msg.Size(), msg.ChannelID, msg.Type)

		// call the callback
		r.callback(&msg, pid)
	}
}

// messagePubKey extracts the public key of the envelope signer from a libp2p message.
// The location of that key depends on the type of the key, see:
// https://github.com/libp2p/specs/blob/master/peer-ids/peer-ids.md
// This reproduces the exact logic of the private function doing the same decoding in libp2p:
// https://github.com/libp2p/go-libp2p-pubsub/blob/ba28f8ecfc551d4d916beb748d3384951bce3ed0/sign.go#L77
func messageSigningID(m *pubsub.Message) (peer.ID, error) {
	var pubk lcrypto.PubKey

	// m.From is the original sender of the message (versus `m.ReceivedFrom` which is the last hop which sent us this message)
	pid, err := peer.IDFromBytes(m.From)
	if err != nil {
		return "", err
	}

	if m.Key == nil {
		// no attached key, it must be extractable from the source ID
		pubk, err = pid.ExtractPublicKey()
		if err != nil {
			return "", fmt.Errorf("cannot extract signing key: %s", err.Error())
		}
		if pubk == nil {
			return "", fmt.Errorf("cannot extract signing key")
		}
	} else {
		pubk, err = lcrypto.UnmarshalPublicKey(m.Key)
		if err != nil {
			return "", fmt.Errorf("cannot unmarshal signing key: %s", err.Error())
		}

		// verify that the source ID matches the attached key
		if !pid.MatchesPublicKey(pubk) {
			return "", fmt.Errorf("bad signing key; source ID %s doesn't match key", pid)
		}
	}

	// the pid either contains or matches the signing pubKey
	return pid, nil
}
