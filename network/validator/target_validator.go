package validator

import (
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/utils/logging"
)

var _ network.MessageValidator = &TargetValidator{}

// TargetValidator filters out messages by target ID
type TargetValidator struct {
	target flow.Identifier
	log    zerolog.Logger
}

var _ network.MessageValidator = (*TargetValidator)(nil)

// ValidateTarget returns a new TargetValidator for the given target id
func ValidateTarget(log zerolog.Logger, target flow.Identifier) network.MessageValidator {
	tv := &TargetValidator{
		target: target,
		log:    log,
	}
	return tv
}

// Validate returns true if the message is intended for the given target ID else it returns false
func (tv *TargetValidator) Validate(msg network.IncomingMessageScope) bool {
	for _, t := range msg.TargetIDs() {
		if tv.target == t {
			return true
		}
	}
	tv.log.Debug().
		Hex("message_target_id", logging.ID(tv.target)).
		Hex("local_node_id", logging.ID(tv.target)).
		Hex("event_id", msg.EventID()).
		Msg("message not intended for target")
	return false
}
