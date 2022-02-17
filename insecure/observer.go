package insecure

import (
	"context"

	"github.com/onflow/flow-go/insecure/proto"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/network"
)

type observer interface {
	observe(context.Context, interface{}, network.Channel, proto.Protocol, uint32, ...flow.Identifier)
}
