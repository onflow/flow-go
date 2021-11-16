package p2p

import (
	p2pstream "github.com/onflow/flow-go/network/p2p/stream"
)

type ProtocolBuilder struct {
	factories p2pstream.StreamFactoryList
}

func (p *ProtocolBuilder) WithStreamFactory(factory p2pstream.StreamFactory) *ProtocolBuilder {

}

func (p *ProtocolBuilder) RegisterStreamHandlers() {

}
