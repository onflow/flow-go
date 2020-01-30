package libp2p

import (
	"context"
	"time"

	"github.com/libp2p/go-libp2p-core/discovery"
	"github.com/libp2p/go-libp2p-core/peer"
)

type Discovery struct {
}

func (d *Discovery) Advertise(ctx context.Context, ns string, opts ...discovery.Option) (time.Duration, error) {
	var options discovery.Options
	err := options.Apply(opts...)
	if err != nil {
		return 0, err
	}

	return time.Second, nil
}

func (d *Discovery) FindPeers(ctx context.Context, ns string, opts ...discovery.Option) (<-chan peer.AddrInfo, error) {
	var options discovery.Options
	err := options.Apply(opts...)
	if err != nil {
		return nil, err
	}

	ch := make(chan peer.AddrInfo, 1)
	return ch, nil
}

func Options() []discovery.Option {
	return []discovery.Option{discovery.TTL(1 * time.Hour)}
}
