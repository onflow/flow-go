package underlay

import (
	"github.com/ipfs/go-datastore"
	"github.com/libp2p/go-libp2p/core/protocol"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/network/channels"
)

type NoopConduit struct{}

var _ network.Conduit = (*NoopConduit)(nil)

func (n *NoopConduit) ReportMisbehavior(network.MisbehaviorReport) {}

func (n *NoopConduit) Publish(event interface{}, targetIDs ...flow.Identifier) error {
	return nil
}

func (n *NoopConduit) Unicast(event interface{}, targetID flow.Identifier) error {
	return nil
}

func (n *NoopConduit) Multicast(event interface{}, num uint, targetIDs ...flow.Identifier) error {
	return nil
}

func (n *NoopConduit) Close() error {
	return nil
}

type NoopEngineRegister struct {
	module.NoopComponent
}

func (n NoopEngineRegister) Register(channel channels.Channel, messageProcessor network.MessageProcessor) (network.Conduit, error) {
	return &NoopConduit{}, nil
}

func (n NoopEngineRegister) RegisterBlobService(channel channels.Channel, store datastore.Batching, opts ...network.BlobServiceOption) (network.BlobService, error) {
	return nil, nil
}

func (n NoopEngineRegister) RegisterPingService(pingProtocolID protocol.ID, pingInfoProvider network.PingInfoProvider) (network.PingService, error) {
	return nil, nil
}

var _ network.EngineRegistry = (*NoopEngineRegister)(nil)
