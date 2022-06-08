package network

import (
	"context"
	"time"

	"github.com/libp2p/go-libp2p-core/peer"

	"github.com/onflow/flow-go/network/message"
)

// PingInfoProvider is the interface used by the PingService to respond to incoming PingRequest with a PingResponse
// populated with the necessary details
type PingInfoProvider interface {
	SoftwareVersion() string
	SealedBlockHeight() uint64
	HotstuffView() uint64
}

type PingService interface {
	Ping(ctx context.Context, peerID peer.ID) (message.PingResponse, time.Duration, error)
}
