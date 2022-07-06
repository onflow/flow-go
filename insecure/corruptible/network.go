package corruptible

import (
	"github.com/ipfs/go-datastore"
	"github.com/libp2p/go-libp2p-core/protocol"

	"github.com/onflow/flow-go/module/irrecoverable"
	flownet "github.com/onflow/flow-go/network"
)

type Network struct {
	flowNetwork flownet.Network // original flow network of the node.
}

var _ flownet.Network = &Network{}

func (n *Network) Start(context irrecoverable.SignalerContext) {
	n.flowNetwork.Start(context)
}

func (n Network) Ready() <-chan struct{} {
	return n.flowNetwork.Ready()
}

func (n Network) Done() <-chan struct{} {
	return n.flowNetwork.Done()
}

func (n Network) Register(channel flownet.Channel, messageProcessor flownet.MessageProcessor) (flownet.Conduit, error) {
	//TODO implement me
	panic("implement me")
}

func (n *Network) RegisterBlobService(channel flownet.Channel, store datastore.Batching, opts ...flownet.BlobServiceOption) (flownet.BlobService,
	error) {
	return n.RegisterBlobService(channel, store, opts...)
}

func (n Network) RegisterPingService(pingProtocolID protocol.ID, pingInfoProvider flownet.PingInfoProvider) (flownet.PingService, error) {
	return n.RegisterPingService(pingProtocolID, pingInfoProvider)
}
