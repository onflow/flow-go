package encodable

import (
	"github.com/onflow/flow-go/model/flow"
)

// DKG is the encoding format for protocol.DKG
// TODO consolidate with encodable in flow/epoch.go
type DKG struct {
	Size         uint
	GroupKey     RandomBeaconPubKey
	Participants map[flow.Identifier]flow.DKGParticipant
}
