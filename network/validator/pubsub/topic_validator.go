package validator

import (
	"context"
	"fmt"

	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/network/channels"
	"github.com/onflow/flow-go/network/codec"
	"github.com/onflow/flow-go/network/slashing"

	"github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/network/message"
	"github.com/onflow/flow-go/network/validator"
	_ "github.com/onflow/flow-go/utils/binstat"
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
	Message           *message.Message
	DecodedMsgPayload interface{}
	From              peer.ID
}

// TopicValidator is the topic validator that is registered with libP2P whenever a flow libP2P node subscribes to a topic.
// The TopicValidator will decode and perform validation on the raw pubsub message.
func TopicValidator(log zerolog.Logger, c network.Codec, slashingViolationsConsumer slashing.ViolationsConsumer, peerFilter func(peer.ID) error, validators ...validator.PubSubMessageValidator) pubsub.ValidatorEx {
	log = log.With().
		Str("component", "libp2p_node_topic_validator").
		Logger()

	return func(ctx context.Context, receivedFrom peer.ID, rawMsg *pubsub.Message) pubsub.ValidationResult {
		var msg message.Message
		// convert the incoming raw message payload to Message type
		//bs := binstat.EnterTimeVal(binstat.BinNet+":wire>1protobuf2message", int64(len(rawMsg.Data)))
		err := msg.Unmarshal(rawMsg.Data)
		//binstat.Leave(bs)
		if err != nil {
			return pubsub.ValidationReject
		}

		from, err := messageSigningID(rawMsg)
		if err != nil {
			return pubsub.ValidationReject
		}

		if err := peerFilter(from); err != nil {
			log.Warn().
				Err(err).
				Str("peer_id", from.String()).
				Hex("sender", msg.OriginID).
				Bool("suspicious", true).
				Msg("filtering message from un-allowed peer")
			return pubsub.ValidationReject
		}

		// Convert message payload to a known message type
		decodedMsgPayload, err := c.Decode(msg.Payload)
		switch {
		case err == nil:
			break
		case codec.IsErrUnknownMsgCode(err):
			// slash peer if message contains unknown message code byte
			slashingViolationsConsumer.OnUnknownMsgTypeError(violation(from, msg, err))
			return pubsub.ValidationReject
		case codec.IsErrMsgUnmarshal(err):
			// slash if peer sent a message that could not be marshalled into the message type denoted by the message code byte
			slashingViolationsConsumer.OnInvalidMsgError(violation(from, msg, err))
			return pubsub.ValidationReject
		default:
			// unexpected error condition. this indicates there's a bug
			// don't crash as a result of external inputs since that creates a DoS vector.
			log.
				Error().
				Err(fmt.Errorf("unexpected error while decoding message: %w", err)).
				Str("peer_id", from.String()).
				Hex("sender", msg.OriginID).
				Bool("suspicious", true).
				Msg("rejecting message")
			return pubsub.ValidationReject
		}

		rawMsg.ValidatorData = TopicValidatorData{
			Message:           &msg,
			DecodedMsgPayload: decodedMsgPayload,
			From:              from,
		}

		result := pubsub.ValidationAccept
		for _, validator := range validators {
			switch res := validator(from, decodedMsgPayload); res {
			case pubsub.ValidationReject:
				return res
			case pubsub.ValidationIgnore:
				result = res
			}
		}

		return result
	}
}

func violation(pid peer.ID, msg message.Message, err error) *slashing.Violation {
	return &slashing.Violation{
		PeerID:    pid.String(),
		MsgType:   msg.Type,
		Channel:   channels.Channel(msg.ChannelID),
		IsUnicast: false,
		Err:       err,
	}
}
