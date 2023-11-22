package netconf

import "time"

type ConnectionManagerConfig struct {
	// HighWatermark and LowWatermark govern the number of connections are maintained by the ConnManager.
	// When the peer count exceeds the HighWatermark, as many peers will be pruned (and
	// their connections terminated) until LowWatermark peers remain. In other words, whenever the
	// peer count is x > HighWatermark, the ConnManager will prune x - LowWatermark peers.
	// The pruning algorithm is as follows:
	// 1. The ConnManager will not prune any peers that have been connected for less than GracePeriod.
	// 2. The ConnManager will not prune any peers that are protected.
	// 3. The ConnManager will sort the peers based on their number of streams and direction of connections, and
	// prunes the peers with the least number of streams. If there are ties, the peer with the incoming connection
	// will be pruned. If both peers have incoming connections, and there are still ties, one of the peers will be
	// pruned at random.
	// Algorithm implementation is in https://github.com/libp2p/go-libp2p/blob/master/p2p/net/connmgr/connmgr.go#L262-L318
	HighWatermark int `mapstructure:"libp2p-high-watermark"` // naming from libp2p
	LowWatermark  int `mapstructure:"libp2p-low-watermark"`  // naming from libp2p

	// SilencePeriod is the time to wait before start pruning connections.
	SilencePeriod time.Duration `mapstructure:"libp2p-silence-period"` // naming from libp2p
	// GracePeriod is the time to wait before pruning a new connection.
	GracePeriod time.Duration `mapstructure:"libp2p-grace-period"` // naming from libp2p
}
