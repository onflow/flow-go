package validator

import (
	"context"
	"fmt"

	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/network/channels"
	"github.com/onflow/flow-go/network/message"
	"github.com/onflow/flow-go/network/p2p"
	p2plogging "github.com/onflow/flow-go/network/p2p/logging"
	"github.com/onflow/flow-go/network/validator"
	_ "github.com/onflow/flow-go/utils/binstat"
	"github.com/onflow/flow-go/utils/logging"
)

// messagePubKey extracts the public key of the envelope signer from a libp2p message.
// The location of that key depends on the type of the key, see:
// https://github.com/libp2p/specs/blob/master/peer-ids/peer-ids.md
// This reproduces the exact logic of the private function doing the same decoding in libp2p:
// https://github.com/libp2p/go-libp2p-pubsub/blob/ba28f8ecfc551d4d916beb748d3384951bce3ed0/sign.go#L77
func messageSigningID(m *pubsub.Message) (peer.ID, error) {
	var pubk crypto.PubKey

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
		pubk, err = crypto.UnmarshalPublicKey(m.Key)
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

// TopicValidatorData includes information about the message being sent.
type TopicValidatorData struct {
	Message *message.Message
	From    peer.ID
}

// TopicValidator is the topic validator that is registered with libP2P whenever a flow libP2P node subscribes to a topic.
// The TopicValidator will perform validation on the raw pubsub message.
func TopicValidator(log zerolog.Logger, peerFilter func(peer.ID) error, validators ...validator.PubSubMessageValidator) p2p.TopicValidatorFunc {
	log = log.With().
		Str("component", "libp2p-node-topic-validator").
		Logger()

	return func(ctx context.Context, receivedFrom peer.ID, rawMsg *pubsub.Message) p2p.ValidationResult {
		var msg message.Message
		// convert the incoming raw message payload to Message type
		// bs := binstat.EnterTimeVal(binstat.BinNet+":wire>1protobuf2message", int64(len(rawMsg.Data)))
		err := msg.Unmarshal(rawMsg.Data)
		// binstat.Leave(bs)
		if err != nil {
			return p2p.ValidationReject
		}

		from, err := messageSigningID(rawMsg)
		if err != nil {
			return p2p.ValidationReject
		}

		lg := log.With().
			Str("peer_id", p2plogging.PeerId(from)).
			Str("topic", rawMsg.GetTopic()).
			Int("raw_msg_size", len(rawMsg.Data)).
			Int("msg_size", msg.Size()).
			Logger()

		// verify sender is a known peer
		if err := peerFilter(from); err != nil {
			lg.Warn().
				Err(err).
				Bool(logging.KeySuspicious, true).
				Msg("filtering message from un-allowed peer")
			return p2p.ValidationReject
		}

		// verify ChannelID in message matches the topic over which the message was received
		topic := channels.Topic(rawMsg.GetTopic())
		actualChannel, ok := channels.ChannelFromTopic(topic)
		if !ok {
			lg.Warn().
				Bool(logging.KeySuspicious, true).
				Msg("could not convert topic to channel")
			return p2p.ValidationReject
		}

		lg = lg.With().Str("channel", msg.ChannelID).Logger()

		channel := channels.Channel(msg.ChannelID)
		if channel != actualChannel {
			log.Warn().
				Str("actual_channel", actualChannel.String()).
				Bool(logging.KeySuspicious, true).
				Msg("channel id in message does not match pubsub topic")
			return p2p.ValidationReject
		}

		rawMsg.ValidatorData = TopicValidatorData{
			Message: &msg,
			From:    from,
		}

		result := p2p.ValidationAccept
		for _, v := range validators {
			switch res := v(from, &msg); res {
			case p2p.ValidationReject:
				return res
			case p2p.ValidationIgnore:
				result = res
			}
		}

		return result
	}
}
