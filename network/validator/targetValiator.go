package validator

import (
	"bytes"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/network/message"
)

var _ network.MessageValidator = &TargetValidator{}

// TargetValidator filters out messages by target ID
type TargetValidator struct {
	target []byte
	log    zerolog.Logger
}

// ValidateTarget returns a new TargetValidator for the given target id
func ValidateTarget(log zerolog.Logger, target flow.Identifier) network.MessageValidator {
	tv := &TargetValidator{
		target: target[:],
		log:    log,
	}
	return tv
}

// Validate returns true if the message is intended for the given target ID else it returns false
func (tv *TargetValidator) Validate(msg message.Message) bool {
	for _, t := range msg.TargetIDs {
		if bytes.Equal(tv.target, t) {
			return true
		}
	}
	tv.log.Debug().Hex("target", tv.target).Hex("event_id", msg.EventID).Msg("message not intended for target")
	return false
}
