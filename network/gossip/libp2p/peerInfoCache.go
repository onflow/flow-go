package libp2p

import (
	"fmt"

	lru "github.com/hashicorp/golang-lru"
	"github.com/libp2p/go-libp2p-core/peer"

	"github.com/onflow/flow-go/model/flow"
)

var infoCache *peerInfoCache

type peerInfoCache struct {
	*lru.Cache
}

func InitializePeerInfoCache() error {
	cache, err := lru.New(100)
	if err != nil {
		return fmt.Errorf("failed to initialize PeerInfo cache: %w", err)
	}
	infoCache = &peerInfoCache{
		Cache : cache,
	}
	return nil
}

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
