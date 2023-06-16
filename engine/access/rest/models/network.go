package models

import (
	"github.com/onflow/flow-go/access"

	accessproto "github.com/onflow/flow/protobuf/go/flow/access"
)

func (t *NetworkParameters) Build(params *access.NetworkParameters) {
	t.ChainId = params.ChainID.String()
}

func (t *NetworkParameters) BuildFromGrpc(response *accessproto.GetNetworkParametersResponse) {
	t.ChainId = response.ChainId
}
