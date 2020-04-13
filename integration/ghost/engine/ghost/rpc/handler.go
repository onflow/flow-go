package rpc

import (
	"github.com/rs/zerolog"

	protobuf "github.com/dapperlabs/flow-go/integration/ghost/protobuf"
	"github.com/dapperlabs/flow-go/state/protocol"
)

type Handler struct {
	log   zerolog.Logger
	state protocol.State
}

var _ protobuf.GhostSenderServer = Handler{}

func NewHandler(log zerolog.Logger, state protocol.State) *Handler {
	return &Handler{
		log:   log,
		state: state,
	}
}
