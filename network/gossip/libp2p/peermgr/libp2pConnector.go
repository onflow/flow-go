package peermgr

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	"github.com/hashicorp/go-multierror"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/peer"
	discovery "github.com/libp2p/go-libp2p-discovery"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/network/gossip/libp2p"
)

type libp2pConnector struct {
	backoffConnector *discovery.BackoffConnector
	host host.Host
}

var _ Connector = &libp2pConnector{}

func NewLibp2pConnector(host host.Host) (*libp2pConnector, error) {
	connector, err := defaultLibp2pBackoffConnector(host)
	if err != nil {
		return nil, err
	}
	return &libp2pConnector{
		backoffConnector: connector,
		host: host,
	}, nil
}

func (l *libp2pConnector) ConnectPeers(ctx context.Context, ids flow.IdentityList) error {
	// create a channel of peer.AddrInfo as expected by the connector
	peerCh := make(chan peer.AddrInfo, len(ids))

	var errs *multierror.Error

	// convert flow identity to peer.AddrInfo and stuff it into the peerCh channel
	for _, id := range ids {
		peerInfo, err := libp2p.PeerInfoFromID(*id)
		if err != nil {
			errs = multierror.Append(errs, fmt.Errorf(""))
		}
		peerCh <- peerInfo
	}

	// ask the connector to connect to all the peers
	l.backoffConnector.Connect(ctx, peerCh)

	// return an error if any of the flow ids could not be converted to peer.AddressInfo
	errCnt := errs.Len()
	if errCnt > 0 {
		return fmt.Errorf("failed to convert Flow Identity to libp2p peerInfo: %w", errs)
	}

	return nil
}

func (l *libp2pConnector) DisconnectPeers(ctx context.Context, ids flow.IdentityList) error {
	return nil
}

func defaultLibp2pBackoffConnector(host host.Host) (*discovery.BackoffConnector, error){
	rngSrc := rand.NewSource(rand.Int63())
	minBackoff, maxBackoff := time.Second*10, time.Hour
	cacheSize := 100
	dialTimeout := time.Minute * 2
	backoff := discovery.NewExponentialBackoff(minBackoff, maxBackoff, discovery.FullJitter, time.Second, 5.0, 0, rand.New(rngSrc))
	backoffConnector, err := discovery.NewBackoffConnector(host, cacheSize, dialTimeout, backoff)
	if err != nil {
		return nil, fmt.Errorf("failed to create backoff connector: %w", err)
	}
	return backoffConnector, nil
}

