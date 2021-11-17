package unicast

import (
	"fmt"

	"github.com/libp2p/go-libp2p-core/host"
	libp2pnet "github.com/libp2p/go-libp2p-core/network"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/network/p2p"
)

type ProtocolBuilder struct {
	logger         zerolog.Logger
	host           host.Host
	defaultUnicast Protocol
	rootBlockId    flow.Identifier
}

func NewProtocolBuilder(logger zerolog.Logger, host host.Host, rootBlockId flow.Identifier, defaultHandler libp2pnet.StreamHandler) *ProtocolBuilder {

	return &ProtocolBuilder{
		logger:      logger,
		host:        host,
		rootBlockId: rootBlockId,
		defaultUnicast: &PlainStream{
			protocolId: p2p.FlowProtocolID(rootBlockId),
			handler:    defaultHandler,
		},
	}
}

func (builder *ProtocolBuilder) Register(unicasts []ProtocolName) error {
	builder.host.SetStreamHandler(builder.defaultUnicast.ProtocolId(), builder.defaultUnicast.Handler())

	for _, u := range unicasts {
		factory, err := ToProtocolFactory(u)
		if err != nil {
			return fmt.Errorf("could not translate protocol name into factory: %w", err)
		}

		protocol := factory(builder.logger, builder.rootBlockId, builder.defaultUnicast.Handler())
		builder.host.SetStreamHandler(protocol.ProtocolId(), protocol.Handler())
	}

	return nil
}
