package unicast

import (
	"github.com/libp2p/go-libp2p-core/host"
)

type ProtocolBuilder struct {
	host     host.Host
	unicasts []Protocol
}

func (builder *ProtocolBuilder) WithUnicastProtocol(u Protocol) *ProtocolBuilder {
	builder.unicasts = append(builder.unicasts, u)

	return builder
}

func (builder *ProtocolBuilder) Register() {
	for _, u := range builder.unicasts {
		builder.host.SetStreamHandler(u.ProtocolId(), u.Handler())
	}
}
