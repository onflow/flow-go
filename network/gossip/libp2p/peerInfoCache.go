package libp2p

import (
	"fmt"

	lru "github.com/hashicorp/golang-lru"
	"github.com/libp2p/go-libp2p-core/peer"

	"github.com/onflow/flow-go/model/flow"
)

var infoCache *peerInfoCache

// peerInfoCache caches the peer.AddrInfo created from a flow.Identity
type peerInfoCache struct {
	*lru.Cache
}

func InitializePeerInfoCache() error {
	cache, err := lru.New(100)
	if err != nil {
		return fmt.Errorf("failed to initialize PeerInfo cache: %w", err)
	}
	infoCache = &peerInfoCache{
		Cache: cache,
	}
	return nil
}

// PeerInfoFromID converts the flow.Identity to peer.AddrInfo and caches the result
// A node in flow is defined by a flow.Identity while it is defined by a peer.AddrInfo in libp2p.
// flow.Identity           ---> peer.AddrInfo
//    |-- Address          --->   |-- []multiaddr.Multiaddr
//    |-- NetworkPublicKey --->   |-- ID
func PeerInfoFromID(id flow.Identity) (peer.AddrInfo, error) {

	cacheKey := id.NodeID
	addrFromCache, found := infoCache.Get(cacheKey)
	if found {
		return addrFromCache.(peer.AddrInfo), nil
	}

	nodeAddress, err := nodeAddressFromIdentity(id)
	if err != nil {
		return peer.AddrInfo{}, err
	}

	addr, err := GetPeerInfo(nodeAddress)
	if err != nil {
		return peer.AddrInfo{}, err
	}

	_ = infoCache.Add(cacheKey, addr)

	return addr, nil
}
