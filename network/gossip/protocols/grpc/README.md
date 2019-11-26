# protocols
`import "github.com/dapperlabs/flow-go/network/gossip/protocols/grpc"`

* [Overview](#pkg-overview)
* [Index](#pkg-index)

## <a name="pkg-overview">Overview</a>
A protocol is the underlying network layer that the gossip layer uses in order to send and receive messages. Every protocol must implement the ServePlacer interface:
```
type ServePlacer interface {
	// Serve starts serving a new connection
	Serve(net.Listener)
	// Place places a message for sending according to its gossip mode
	Place(context.Context, string, *messages.GossipMessage, bool, Mode) (*messages.GossipReply, error)
}
```
Currently, the gossip layer uses gRPC as its underlying network layer, which is implemented in [grpcServePlacer](https://github.com/dapperlabs/flow-go/tree/master/network/gossip/protocols/grpcServePlacer.go)

grpcServePlacer maintains persistent connections with a specific set of peers in the network layer known as the static fanout. This caching is implemented in [CacheDialer](https://github.com/dapperlabs/flow-go/tree/master/network/gossip/protocols/cachedialer.go).

Connections to peers outside the static fanout are also cached. This is implemented in [PeerQueue](https://github.com/dapperlabs/flow-go/tree/master/network/gossip/protocols/peerqueue.go). In peerqueue, cached connections are prioritized based on how often they are used, with the least used connections being dropped first to make space for new connections.





## <a name="pkg-index">Index</a>
* [type CacheDialer](#CacheDialer)
  * [func NewCacheDialer(dynamicFanoutQueueSize int) (*CacheDialer, error)](#NewCacheDialer)
* [type Gserver](#Gserver)
  * [func NewGServer(n Node) (*Gserver, error)](#NewGServer)
  * [func (gs *Gserver) Place(ctx context.Context, addr string, msg *messages.GossipMessage, isSynchronous bool, mode gossip.Mode) (*messages.GossipReply, error)](#Gserver.Place)
  * [func (gs *Gserver) QueueService(ctx context.Context, msg *messages.GossipMessage) (*messages.GossipReply, error)](#Gserver.QueueService)
  * [func (gs *Gserver) Serve(listener net.Listener)](#Gserver.Serve)
  * [func (gs *Gserver) StreamQueueService(saq messages.MessageReceiver_StreamQueueServiceServer) error](#Gserver.StreamQueueService)
* [type Node](#Node)
* [type PeerQueue](#PeerQueue)
  * [func NewPeerQueue(size int) (*PeerQueue, error)](#NewPeerQueue)


#### <a name="pkg-files">Package files</a>
[cachedialer.go](https://github.com/dapperlabs/flow-go/tree/master/network/gossip/protocols/cachedialer.go) [grpcServePlacer.go](https://github.com/dapperlabs/flow-go/tree/master/network/gossip/protocols/grpcServePlacer.go) [peerqueue.go](https://github.com/dapperlabs/flow-go/tree/master/network/gossip/protocols/peerqueue.go)






## <a name="CacheDialer">type</a> [CacheDialer](https://github.com/dapperlabs/flow-go/tree/master/network/gossip/protocols/cachedialer.go?s=401:605#L18)
``` go
type CacheDialer struct {
    // contains filtered or unexported fields
}

```
CacheDialer deals with the dynamic fanout of the gossip network. It returns a cached stream, or if not found, creates and
caches a new stream







### <a name="NewCacheDialer">func</a> [NewCacheDialer](https://github.com/dapperlabs/flow-go/tree/master/network/gossip/protocols/cachedialer.go?s=698:767#L25)
``` go
func NewCacheDialer(dynamicFanoutQueueSize int) (*CacheDialer, error)
```
NewCacheDialer returns a new cache dialer with the determined dynamic fanout queue size





## <a name="Gserver">type</a> [Gserver](https://github.com/dapperlabs/flow-go/tree/master/network/gossip/protocols/grpcServePlacer.go?s=847:951#L36)
``` go
type Gserver struct {
    // contains filtered or unexported fields
}

```
Gserver represents a gRPC server and a client







### <a name="NewGServer">func</a> [NewGServer](https://github.com/dapperlabs/flow-go/tree/master/network/gossip/protocols/grpcServePlacer.go?s=998:1039#L43)
``` go
func NewGServer(n Node) (*Gserver, error)
```
NewGServer returns a new Gserver instance





### <a name="Gserver.Place">func</a> (\*Gserver) [Place](https://github.com/dapperlabs/flow-go/tree/master/network/gossip/protocols/grpcServePlacer.go?s=2899:3055#L112)
``` go
func (gs *Gserver) Place(ctx context.Context, addr string, msg *messages.GossipMessage, isSynchronous bool, mode gossip.Mode) (*messages.GossipReply, error)
```
Place places a message for sending according to the gossip mode




### <a name="Gserver.QueueService">func</a> (\*Gserver) [QueueService](https://github.com/dapperlabs/flow-go/tree/master/network/gossip/protocols/grpcServePlacer.go?s=1510:1622#L60)
``` go
func (gs *Gserver) QueueService(ctx context.Context, msg *messages.GossipMessage) (*messages.GossipReply, error)
```
QueueService is invoked remotely using the gRPC stub,
it receives a message from a remote node and places it inside the local nodes queue
In current version of Gossip, StreamQueueService is utilized for direct 1-to-1 gossips




### <a name="Gserver.Serve">func</a> (\*Gserver) [Serve](https://github.com/dapperlabs/flow-go/tree/master/network/gossip/protocols/grpcServePlacer.go?s=2595:2642#L103)
``` go
func (gs *Gserver) Serve(listener net.Listener)
```
Serve starts serving a new connection




### <a name="Gserver.StreamQueueService">func</a> (\*Gserver) [StreamQueueService](https://github.com/dapperlabs/flow-go/tree/master/network/gossip/protocols/grpcServePlacer.go?s=1856:1954#L66)
``` go
func (gs *Gserver) StreamQueueService(saq messages.MessageReceiver_StreamQueueServiceServer) error
```
StreamQueueService receives sync data from stream and places is in queue
In current version of Gossip, StreamQueueService is utilized for 1-to-many and 1-to-all gossips




## <a name="Node">type</a> [Node](https://github.com/dapperlabs/flow-go/tree/master/network/gossip/protocols/grpcServePlacer.go?s=463:581#L24)
``` go
type Node interface {
    QueueService(ctx context.Context, msg *messages.GossipMessage) (*messages.GossipReply, error)
}
```
Node interface defines the functions that any network node should have so that it can use Gserver










## <a name="PeerQueue">type</a> [PeerQueue](https://github.com/dapperlabs/flow-go/tree/master/network/gossip/protocols/peerqueue.go?s=263:306#L10)
``` go
type PeerQueue struct {
    // contains filtered or unexported fields
}

```
PeerQueue keeps track prioritizing the streams based on the last time they used
it operates based on an LRU cache and discards the least recently used connection







### <a name="NewPeerQueue">func</a> [NewPeerQueue](https://github.com/dapperlabs/flow-go/tree/master/network/gossip/protocols/peerqueue.go?s=375:422#L15)
``` go
func NewPeerQueue(size int) (*PeerQueue, error)
```
NewPeerQueue constructs a new PeerQueue with the specified size








