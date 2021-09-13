package validator

import (
	"context"
	"fmt"

	"github.com/libp2p/go-libp2p-core/crypto"
	"github.com/libp2p/go-libp2p-core/peer"
	pubsub "github.com/libp2p/go-libp2p-pubsub"

	"github.com/onflow/flow-go/network/message"
)

// TODO: break circular dependency and remove duplicate function
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

type MessageValidator func(context.Context, peer.ID, *message.Message) pubsub.ValidationResult

type ValidatorData struct {
	Message *message.Message
	From    peer.ID
}

func TopicValidator(validators ...MessageValidator) pubsub.ValidatorEx {
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

		rawMsg.ValidatorData = ValidatorData{
			Message: &msg,
			From:    from,
		}

		result := pubsub.ValidationAccept
		for _, validator := range v.validators {
			switch res := validator(ctx, from, &msg); res {
			case pubsub.ValidationReject:
				return res
			case pubsub.ValidationIgnore:
				result = res
			}
		}

		return result
	}
}
