package connection

import (
	"fmt"
	"time"

	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"

	"github.com/onflow/flow-go/module"
)

type MockLip2pCollector struct {
}

func NewMockLip2pResourceCollector() *MockLip2pCollector {
	return &MockLip2pCollector{}
}

var _ module.LibP2PMetrics = (*MockLip2pCollector)(nil)

func (m *MockLip2pCollector) OnIncomingRpcAcceptedFully() {}

func (m *MockLip2pCollector) OnIncomingRpcAcceptedOnlyForControlMessages() {}

func (m *MockLip2pCollector) OnIncomingRpcRejected() {}

func (m *MockLip2pCollector) OnIWantReceived(count int) {}

func (m *MockLip2pCollector) OnIHaveReceived(count int) {}

func (m *MockLip2pCollector) OnGraftReceived(count int) {}

func (m *MockLip2pCollector) OnPruneReceived(count int) {}

func (m *MockLip2pCollector) OnPublishedGossipMessagesReceived(count int) {}

func (m *MockLip2pCollector) DNSLookupDuration(duration time.Duration) {}

func (m *MockLip2pCollector) OnDNSCacheMiss() {}

func (m *MockLip2pCollector) OnDNSCacheHit() {}

func (m *MockLip2pCollector) OnDNSCacheInvalidated() {}

func (m *MockLip2pCollector) OnDNSLookupRequestDropped() {}

func (m *MockLip2pCollector) RoutingTablePeerAdded() {}

func (m *MockLip2pCollector) RoutingTablePeerRemoved() {}

func (m *MockLip2pCollector) OutboundConnections(connectionCount uint) {}

func (m *MockLip2pCollector) InboundConnections(connectionCount uint) {}

func (m *MockLip2pCollector) AllowConn(dir network.Direction, usefd bool) {
	fmt.Println("AllowConn")
}

func (m *MockLip2pCollector) BlockConn(dir network.Direction, usefd bool) {
	fmt.Println("BlockConn")
}

func (m *MockLip2pCollector) AllowStream(p peer.ID, dir network.Direction) {

}

func (m *MockLip2pCollector) BlockStream(p peer.ID, dir network.Direction) {

}

func (m *MockLip2pCollector) AllowPeer(p peer.ID) {

}

func (m *MockLip2pCollector) BlockPeer(p peer.ID) {

}

func (m *MockLip2pCollector) AllowProtocol(proto protocol.ID) {

}

func (m *MockLip2pCollector) BlockProtocol(proto protocol.ID) {

}

func (m *MockLip2pCollector) BlockProtocolPeer(proto protocol.ID, p peer.ID) {

}

func (m *MockLip2pCollector) AllowService(svc string) {

}

func (m *MockLip2pCollector) BlockService(svc string) {

}

func (m *MockLip2pCollector) BlockServicePeer(svc string, p peer.ID) {

}

func (m *MockLip2pCollector) AllowMemory(size int) {

}

func (m *MockLip2pCollector) BlockMemory(size int) {

}
