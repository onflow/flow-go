package protocols

import (
	"context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/proto/gossip/messages"
)

// TestAddPeer tests peer adding (makes sure that once a stream is added, it can be
// found in the cache
func TestAddPeer(t *testing.T) {

	address := "localhost:1337"

	pq, err := NewPeerQueue(10)
	require.Nil(t, err, "could not create peerqueue")

	assert.True(t, !pq.contains(address), "PeerQueue should not contain the address before insertion")

	pq.add(address, &dummyStream{})

	assert.True(t, pq.contains(address), "PeerQueue should contain the address after insertion")

}

// TestGetPeer makes sure that once an address is found inside the cache, that it
// is retrievable
func TestGetPeer(t *testing.T) {

	pq, err := NewPeerQueue(10)
	require.Nil(t, err, "could not create peerqueue")

	address := "localhost:1337"

	_, ok := pq.get(address)
	assert.False(t, ok, "PeerQueue should not be able to get the clients stream")

	pq.add(address, &dummyStream{})

	_, ok = pq.get(address)
	assert.True(t, ok, "PeerQueue should be able to get the clients stream")
}

// TestContainsPeer makes sure that once an address is found inside the cache,
// that it can be verified with contains
func TestContainsPeer(t *testing.T) {

	pq, err := NewPeerQueue(10)
	require.Nil(t, err, "could not create peerqueue")

	address := "localhost:1337"

	assert.False(t, pq.contains(address), "PeerQueue should not contain the clients stream")

	pq.add(address, &dummyStream{})

	assert.True(t, pq.contains(address), "PeerQueue should be able to get the clients stream")
}

// TestPopInAction makes sure that once the queue is filled, then when another
// address is pushed, the least used address will be removed from the queue
func TestPopInAction(t *testing.T) {
	pq, err := NewPeerQueue(10)
	require.Nil(t, err, "could not create peerqueue")

	oldAddress := "localhost:8080"
	newAddress := "localhost:8090"

	addresses := []string{
		"localhost:8081",
		"localhost:8082",
		"localhost:8083",
		"localhost:8084",
		"localhost:8085",
		"localhost:8086",
		"localhost:8087",
		"localhost:8088",
		"localhost:8089",
	}

	pq.add(oldAddress, &dummyStream{})

	// Fill the queue completely
	for _, address := range addresses {
		pq.add(address, &dummyStream{})
	}

	// The queue should still contain the first element
	assert.True(t, pq.contains(oldAddress), "queue should contain the last element before another push")

	// Add another new address to the queue, the queue should pop to provide space
	// for the new address
	pq.add(newAddress, &dummyStream{})

	// The firstly added address should not be found in cache
	assert.False(t, pq.contains(oldAddress), "queue should not contain the last element after push")

	// The newly addded address should be found in cache
	assert.True(t, pq.contains(newAddress), "queue should contain the newest element after push")
}

// TestRemove makes sure that when an address is removed it will be taken out of
// cache
func TestRemove(t *testing.T) {
	address := "localhost:1337"
	pq, err := NewPeerQueue(10)
	require.Nil(t, err, "could not create peerqueue")

	pq.add(address, &dummyStream{})

	assert.True(t, pq.contains(address), "cache should contain the added address")

	pq.remove(address)

	assert.False(t, pq.contains(address), "cache should not contain the added address")
}

// dummyStream implements clientStream and can be used as an argument
type dummyStream struct {
	cs grpc.ClientStream
}

func (ds *dummyStream) Header() (metadata.MD, error) {
	return nil, nil
}

func (ds *dummyStream) Trailer() metadata.MD {
	return nil
}

func (ds *dummyStream) CloseSend() error {
	return nil
}

func (ds *dummyStream) Context() context.Context {
	return nil
}

func (ds *dummyStream) SendMsg(m interface{}) error {
	return nil
}

func (ds *dummyStream) RecvMsg(m interface{}) error {
	return nil
}

func (ds *dummyStream) Recv() (*messages.GossipReply, error) {
	return nil, nil
}

func (ds *dummyStream) Send(*messages.GossipMessage) error {
	return nil
}
