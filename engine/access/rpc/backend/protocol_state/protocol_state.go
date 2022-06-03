package protocol_state

import (
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
	"github.com/rs/zerolog"
)

type ProtocolState struct {
	BlockHeaders

	State   protocol.State
	ChainID flow.ChainID
	Log     zerolog.Logger
}

func (p *ProtocolState) SetBlockHeader(headers storage.Headers, state protocol.State) {

	p.BlockHeaders = BlockHeaders{
		headers: nil,
		state:   nil,
	}

}
