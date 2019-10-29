package protocols

import (
	"context"
	"fmt"
	"sync"

	"github.com/pkg/errors"
	"google.golang.org/grpc"
	"google.golang.org/grpc/encoding/gzip"

	"github.com/dapperlabs/flow-go/network/gossip"
	"github.com/dapperlabs/flow-go/proto/gossip/messages"
)

// CacheDialer deals with the dynamic fanout of the gossip network. It returns a cached stream, or if not found, creates and
// caches a new stream
type CacheDialer struct {
	mu           sync.Mutex
	pq           *PeerQueue              //LRU cache of the dynamic fanout of the gnode
	staticFanout map[string]clientStream //static fanout of the gnode
}

// NewCacheDialer returns a new cache dialer with the determined dynamic fanout queue size
func NewCacheDialer(dynamicFanoutQueueSize int) (*CacheDialer, error) {
	pq, err := NewPeerQueue(dynamicFanoutQueueSize)
	if err != nil {
		return nil, errors.Wrap(err, "could not create a peerqueue for caching")
	}

	return &CacheDialer{
		pq:           pq,
		staticFanout: make(map[string]clientStream),
	}, nil
}

// removeStream removes the input stream identified by the address
func (cd *CacheDialer) removeStream(address string) {
	cd.mu.Lock()
	defer cd.mu.Unlock()

	cd.pq.remove(address)
	delete(cd.staticFanout, address)
}

// Dial returns a cached stream connected with the specified address if found,
// otherwise it creates a new one and caches it
func (cd *CacheDialer) dial(address string, isSynchronous bool, mode gossip.Mode) (clientStream, error) {
	cd.mu.Lock()
	defer cd.mu.Unlock()

	var stream clientStream

	// We first check to see if the address is part of the static fanout of the node
	if stream, ok := cd.staticFanout[address]; ok {
		return stream, nil
	}

	// If the address is not part of the static fanout of the node, we check the dynamic fanout cache
	if mode == gossip.ModeOneToMany && cd.pq.contains(address) {
		var ok bool
		// retrieving stream from the cache of dynamic fanout
		stream, ok = cd.pq.get(address)

		if !ok {
			return nil, fmt.Errorf("could not get client's stream in cache: %v", ok)
		}
	} else {
		// otherwise create a new stream
		conn, err := grpc.Dial(address, grpc.WithInsecure())
		if err != nil {
			return nil, fmt.Errorf("could not connect to %s: %v", address, err)
		}
		client := messages.NewMessageReceiverClient(conn)

		if isSynchronous {
			stream, err = client.StreamSyncQueue(context.Background(), grpc.UseCompressor(gzip.Name))
		} else {
			stream, err = client.StreamAsyncQueue(context.Background(), grpc.UseCompressor(gzip.Name))
		}

		if err != nil {
			return nil, fmt.Errorf("could not start grpc stream with server %s: %v", address, err)
		}

		switch mode {
		case gossip.ModeOneToMany:
			// if the gossip is one to many, the stream is cached for further gossips
			cd.pq.add(address, stream)
		case gossip.ModeOneToAll:
			// if the gossip is one to all, the stream should be saved as a static fanout, since in
			// one to all a node only dials its static fanouts.
			cd.staticFanout[address] = stream
		}

	}
	return stream, nil
}
