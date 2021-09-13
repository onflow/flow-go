package validator

import (
	"context"

	"github.com/libp2p/go-libp2p-core/peer"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/onflow/flow-go/network/message"
)

type MessageValidator func(context.Context, peer.ID, *message.Message) pubsub.ValidationResult

type TopicValidator struct {
	validators []MessageValidator
}

func (v *TopicValidator) Validate(ctx context.Context, receivedFrom peer.ID, rawMsg *pubsub.Message) pubsub.ValidationResult {
	var msg message.Message
	// convert the incoming raw message payload to Message type
	//bs := binstat.EnterTimeVal(binstat.BinNet+":wire>1protobuf2message", int64(len(rawMsg.Data)))
	err := msg.Unmarshal(rawMsg.Data)
	//binstat.Leave(bs)
	if err != nil {
		return pubsub.ValidationReject
	}
	rawMsg.ValidatorData = msg

	from, err := messageSigningID(rawMsg)
	if err != nil {
		return pubsub.ValidationReject
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
