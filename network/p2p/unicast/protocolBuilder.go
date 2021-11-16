package unicast

import (
	"github.com/libp2p/go-libp2p-core/host"
)

type ProtocolBuilder struct {
	host     host.Host
	unicasts UnicastProtocolList
}

func (builder *ProtocolBuilder) WithStreamFactory(factory) *ProtocolBuilder {
	builder.unicasts = append(builder.unicasts, factory)

	return builder
}

func (builder *ProtocolBuilder) Register() {
	for _, u := range builder.unicasts {
		builder.host.SetStreamHandler(u.ProtocolId(), u.Handler())
	}
}
