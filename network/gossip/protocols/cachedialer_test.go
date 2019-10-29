package protocols

import (
	"fmt"
	"io"
	"net"
	"testing"

	"github.com/dapperlabs/flow-go/network/gossip"
	"github.com/dapperlabs/flow-go/proto/gossip/messages"
)

// TestCacheDialerFundamentals tests the fundamental functionality of CacheDialer, such as invalid inputs
func TestCacheDialerFundamentals(t *testing.T) {
	cd, err := NewCacheDialer(20)
	if err != nil {
		t.Errorf("Unexpected error occured when creating a cacheDilaer: %v", err)
	}

	// initialize grpc serveplacer in order to dial it
	config := gossip.NewNodeConfig(nil, "127.0.0.1:50000", []string{}, 0, 10)
	node := gossip.NewNode(config)

	// establishing a new gRPC server for the gNode
	server, err := NewGServer(node)
	if err != nil {
		t.Errorf("Unexpected error could not create Gserver: %v", err)
	}

	// coupling the new gRPC server with the node
	node.SetProtocol(server)

	ln, err := net.Listen("tcp4", "127.0.0.1:52000")
	if err != nil {
		t.Errorf("could not listen to port: %v", "127.0.0.1:52000")
	}
	go node.Serve(ln)

	tt := []struct {
		address string
		sync    bool
		mode    gossip.Mode
		err     error
	}{
		{ // empty address
			address: "",
			err:     fmt.Errorf("non nil"),
		},
		{ // invalid address
			address: "127.0.0.1:99999",
			sync:    true,
			mode:    gossip.ModeOneToAll,
			err:     fmt.Errorf("non nil"),
		},
		{ // all possible valid inputs
			address: "127.0.0.1:52000",
			sync:    false,
			mode:    gossip.ModeOneToAll,
		},
		{
			address: "127.0.0.1:52000",
			sync:    true,
			mode:    gossip.ModeOneToMany,
		},
		{
			address: "127.0.0.1:52000",
			sync:    false,
			mode:    gossip.ModeOneToMany,
		},
		{
			address: "127.0.0.1:52000",
			sync:    true,
			mode:    gossip.ModeOneToOne,
		},
		{
			address: "127.0.0.1:52000",
			sync:    false,
			mode:    gossip.ModeOneToOne,
		},
	}

	for _, tc := range tt {
		stream, err := cd.dial(tc.address, tc.sync, tc.mode)
		if err != nil && tc.err != nil {
			continue
		}
		if err != nil && tc.err == nil {
			t.Errorf("Dial error mismatch. Expected: nil error, Got: non nil error: %v", err)
		}
		if err == nil && tc.err != nil {
			t.Errorf("Dial error mismatch. Expected: non nil error, Got: nil error")
		}
		if stream == nil {
			t.Errorf("Dial stream return mismatch. Expected: non nil stream, Got: nil stream")
		}
	}
}

// TestCacheDialerCaching tests whether streams are reused as expected
// Static fanout (OneToAll) calls should only use the static fanout cache
// Dynamic fanout (OneToMany) calls should use the static fanout cache if a stream is already available,
// otherwise use its own fanout
func TestCacheDialerCaching(t *testing.T) {
	cd, err := NewCacheDialer(2)
	if err != nil {
		t.Errorf("Unexpected error occured when creating a cacheDilaer: %v", err)
	}

	// initialize grpcServeplacers in order to dial them.
	numInstances := 4

	addresses := []string{
		"127.0.0.1:53000",
		"127.0.0.1:53001",
		"127.0.0.1:53002",
		"127.0.0.1:53003",
	}

	for i := 0; i < numInstances; i++ {
		config := gossip.NewNodeConfig(nil, addresses[i], []string{}, 0, 10)
		node := gossip.NewNode(config)
		// establishing a new gRPC server for the gNode
		server, err := NewGServer(node)
		if err != nil {
			t.Errorf("Unexpected error could not create Gserver: %v", err)
		}

		// coupling the new gRPC server with the node
		node.SetProtocol(server)

		ln, err := net.Listen("tcp4", addresses[i])
		if err != nil {
			t.Errorf("could not listen to port: %v", addresses[i])
		}
		go node.Serve(ln)
	}

	// Using dynamic fanout
	StreamDynamicFanout, err := cd.dial(addresses[0], true, gossip.ModeOneToMany)
	if err != nil {
		t.Errorf("Unexpected error in dial. Expected: nil error, Got: non nil error")
	}

	_, err = cd.dial(addresses[1], true, gossip.ModeOneToMany) //an intermediate dial
	if err != nil {
		t.Errorf("Unexpected error in dial. Expected: nil error, Got: non nil error")
	}

	AnotherStreamDynamicFanout, err := cd.dial(addresses[0], true, gossip.ModeOneToMany)
	if err != nil {
		t.Errorf("Unexpected error in dial. Expected: nil error, Got: non nil error")
	}

	if StreamDynamicFanout != AnotherStreamDynamicFanout {
		t.Errorf("dial returned a different stream in the second call")
	}

	// Using static fanout, the stream should be different than the first streams
	staticFanoutStream, err := cd.dial(addresses[0], true, gossip.ModeOneToAll)
	if err != nil {
		t.Errorf("Unexpected error in dial. Expected: nil error, Got: non nil error")
	}

	if StreamDynamicFanout == staticFanoutStream {
		t.Errorf("Cachedialer return mismatch, Expected: different stream than the first call, Got: same stream")
	}

	// Using dynamic fanout, the stream returned should be the same one as the one from the static fanout
	// since streams in static fanout have a precedence
	AnotherStreamDynamicFanout, err = cd.dial(addresses[0], true, gossip.ModeOneToMany)
	if err != nil {
		t.Errorf("Unexpected error in dial. Expected: nil error, Got: non nil error")
	}
	if AnotherStreamDynamicFanout != staticFanoutStream {
		t.Errorf("Cachedialer return mismatch, Expected: same stream as static fanout, Got: different stream")
	}

	// Testing the LRU cache removal of the least recently used cached stream, by this time address[0:1] are in
	// the cache, but since size of cache is determined as 2 in the beginning of the test, dialing another node rather
	// than 0 or 1 should discard 1, since 0 is used twice after 1, so 1 is the least recent
	_, err = cd.dial(addresses[2], true, gossip.ModeOneToMany) //an intermediate dial
	if err != nil {
		t.Errorf("Unexpected error in dial. Expected: nil error, Got: non nil error")
	}

	if cd.pq.contains(addresses[1]) {
		t.Errorf("Unexpected error in cashdialer. Expected: %v to be removed from cache, Got: is existing", addresses[1])
	}

}

// TestCacheDialerMessaging tests whether streams expire after sending a message
func TestCacheDialerPersistence(t *testing.T) {
	cd, err := NewCacheDialer(20)
	if err != nil {
		t.Errorf("Unexpected error occured when creating a cacheDilaer: %v", err)
	}

	// initialize grpcServeplacer in order to dial it
	config := gossip.NewNodeConfig(nil, "127.0.0.1:50000", []string{}, 0, 10)
	node := gossip.NewNode(config)
	// establishing a new gRPC server for the gNode
	server, err := NewGServer(node)
	if err != nil {
		t.Errorf("Unexpected error could not create Gserver: %v", err)
	}

	// coupling the new gRPC server with the node
	node.SetProtocol(server)

	ln, err := net.Listen("tcp4", "127.0.0.1:54000")
	if err != nil {
		t.Errorf("could not listen to port: %v", "127.0.0.1:54000")
	}
	go node.Serve(ln)

	tt := []struct {
		sync bool
		mode gossip.Mode
	}{
		{
			sync: true,
			mode: gossip.ModeOneToMany,
		},
		{
			sync: false,
			mode: gossip.ModeOneToMany,
		},
		{
			sync: true,
			mode: gossip.ModeOneToAll,
		},
		{
			sync: false,
			mode: gossip.ModeOneToAll,
		},
	}

	for _, tc := range tt {
		stream, err := cd.dial("127.0.0.1:54000", tc.sync, tc.mode)
		if err != nil {
			t.Errorf("Unexpected error in dial. Expected: nil error, Got: non nil error %v", err)
		}

		// sending the first message
		if err := stream.Send(&messages.GossipMessage{}); err != nil {
			t.Errorf("Unexpected error when trying to send a gossip message: %v", err)
		}

		if _, err := stream.Recv(); err != nil {
			t.Errorf("Unexpected error when trying to receive a gossip message: %v", err)
		}

		// sending an empty message to make sure that the connection is kept open after sending a message
		// this may be a trivial test to pass for gRPC, nevertheless, this is a good pattern of test to
		// check the durability of the connections
		err = stream.Send(&messages.GossipMessage{})
		if err == io.EOF {
			t.Errorf("Unexpected error in send. Expected: nil error, Got: EOF")
		}
		_, err = stream.Recv()
		if err == io.EOF {
			t.Errorf("Unexpected error in receive. Expected: nil error, Got: EOF")
		}
	}

}

// A stream (or connection) is called as bad stream if it becomes dead or unresponsive. In that case
// it is being removed from the cache. The purpose of this test is to make sure that a bad stream is
// truely cleaned up from the cache
func TestBadStream(t *testing.T) {
	cd, err := NewCacheDialer(20)
	if err != nil {
		t.Errorf("Unexpected error occured when creating a cacheDialer: %v", err)
	}
	address := "127.0.0.1:55000"
	// initialize grpcServeplacer in order to dial it as a static fanout
	config := gossip.NewNodeConfig(nil, address, []string{}, 0, 10)
	node := gossip.NewNode(config)

	// establishing a new gRPC server for the gNode
	server, err := NewGServer(node)
	if err != nil {
		t.Errorf("Unexpected error could not create Gserver: %v", err)
	}

	// coupling the new gRPC server with the node
	node.SetProtocol(server)

	// provide a listener for the node to serve on
	ln, err := net.Listen("tcp4", address)
	if err != nil {
		t.Errorf("could not listen to port: %v", address)
	}
	go node.Serve(ln)

	// check initial size of both fanout caches
	if len(cd.staticFanout) != 0 || cd.pq.cache.Len() != 0 {
		t.Errorf("Cache size mismatch. Expected: no stream in caches, Got: stream in caches")
	}

	/*
		Static Fanout test
	*/

	// dialing with mode as OneToAll in order to use the static fanout cache
	_, err = cd.dial(address, true, gossip.ModeOneToAll)
	if err != nil {
		t.Errorf("Unexpected error in dial. Expected: nil error, Got: non nil error: %v", err)
	}

	// check size of static fanout cache after dialing once
	if len(cd.staticFanout) != 1 {
		t.Errorf("Cache size mismatch. Expected: 1 stream in cache, Got: %v in cache", len(cd.staticFanout))
	}

	// confirm cache exists in the static fanout
	if _, ok := cd.staticFanout[address]; !ok {
		t.Errorf("Cache content mismatch. Expected: stream with address: %v in cache, Got: no such stream", address)
	}

	// remove the stream
	cd.removeStream(address)

	// check size of dynamic fanout cache after using BadStream
	if len(cd.staticFanout) != 0 {
		t.Errorf("Cache size mismatch. Expected: no stream in cache, Got: %v in cache", len(cd.staticFanout))
	}

	// confirm cache is removed from the static fanout
	if _, ok := cd.staticFanout[address]; ok {
		t.Errorf("Cache content mismatch. Expected: no stream with address: %v in cache, Got: such a stream found", address)
	}

	/*
		Dynamic Fanout test
	*/

	// dialing with mode as OneToMany in order to use the dynamic fanout cache
	_, err = cd.dial(address, true, gossip.ModeOneToMany)
	if err != nil {
		t.Errorf("Unexpected error in dial. Expected: nil error, Got: non nil error: %v", err)
	}

	// check size of dynamic fanout cache after dialing once
	if cd.pq.cache.Len() != 1 {
		t.Errorf("Cache size mismatch. Expected: 1 stream in cache, Got: %v in cache", cd.pq.cache.Len())
	}

	// confirm cache exists in the dynamic fanout
	if !cd.pq.contains(address) {
		t.Errorf("Cache content mismatch. Expected: stream with address: %v in cache, Got: no such stream", address)
	}

	// remove the stream
	cd.removeStream(address)

	// check size of dynamic fanout cache after using BadStream
	if cd.pq.cache.Len() != 0 {
		t.Errorf("Cache size mismatch. Expected: no stream in cache, Got: %v in cache", cd.pq.cache.Len())
	}

	// confirm cache is removed from the dynamic fanout
	if cd.pq.contains(address) {
		t.Errorf("Cache content mismatch. Expected: no stream with address: %v in cache, Got: such a stream found", address)
	}
}
