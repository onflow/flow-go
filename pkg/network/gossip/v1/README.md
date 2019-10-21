

# gnode
`import "github.com/dapperlabs/flow-go/pkg/network/gossip/v1"`

* [Overview](#pkg-overview)
* [Index](#pkg-index)
* [Subdirectories](#pkg-subdirectories)

## <a name="pkg-overview">Overview</a>



## <a name="pkg-index">Index</a>
* [Constants](#pkg-constants)
* [Variables](#pkg-variables)
* [type HandleFunc](#HandleFunc)
* [type MultiRegistry](#MultiRegistry)
  * [func NewMultiRegistry(registries ...Registry) *MultiRegistry](#NewMultiRegistry)
  * [func (mr *MultiRegistry) MessageTypes() map[string]HandleFunc](#MultiRegistry.MessageTypes)
* [type Node](#Node)
  * [func NewNode(msgTypesRegistry Registry, addr string, peers []string, staticFanoutNum int, queueSize int) *Node](#NewNode)
  * [func (n *Node) AsyncGossip(ctx context.Context, payload []byte, recipients []string, msgType string) ([]*shared.GossipReply, error)](#Node.AsyncGossip)
  * [func (n *Node) AsyncQueue(ctx context.Context, msg *shared.GossipMessage) (*shared.GossipReply, error)](#Node.AsyncQueue)
  * [func (n *Node) RegisterFunc(msgType string, f HandleFunc) error](#Node.RegisterFunc)
  * [func (n *Node) Serve(listener net.Listener) error](#Node.Serve)
  * [func (n *Node) SyncGossip(ctx context.Context, payload []byte, recipients []string, msgType string) ([]*shared.GossipReply, error)](#Node.SyncGossip)
  * [func (n *Node) SyncQueue(ctx context.Context, msg *shared.GossipMessage) (*shared.GossipReply, error)](#Node.SyncQueue)
* [type Registry](#Registry)


#### <a name="pkg-files">Package files</a>
[cache.go](https://github.com/dapperlabs/flow-go/tree/master/pkg/network/gossip/v1/cache.go) [database.go](https://github.com/dapperlabs/flow-go/tree/master/pkg/network/gossip/v1/database.go) [gerror.go](https://github.com/dapperlabs/flow-go/tree/master/pkg/network/gossip/v1/gerror.go) [gnode.go](https://github.com/dapperlabs/flow-go/tree/master/pkg/network/gossip/v1/gnode.go) [helper.go](https://github.com/dapperlabs/flow-go/tree/master/pkg/network/gossip/v1/helper.go) [message.go](https://github.com/dapperlabs/flow-go/tree/master/pkg/network/gossip/v1/message.go) [random_selector.go](https://github.com/dapperlabs/flow-go/tree/master/pkg/network/gossip/v1/random_selector.go) [registry.go](https://github.com/dapperlabs/flow-go/tree/master/pkg/network/gossip/v1/registry.go)


## <a name="pkg-constants">Constants</a>
``` go
const DefaultQueueSize = 10
```

## <a name="pkg-variables">Variables</a>
``` go
var (
    ErrTimedOut = errors.New("request timed out")
    ErrInternal = errors.New("gnode internal error")
)
```



## <a name="HandleFunc">type</a> [HandleFunc](https://github.com/dapperlabs/flow-go/tree/master/pkg/network/gossip/v1/registry.go?s=254:315#L10)
``` go
type HandleFunc func(context.Context, []byte) ([]byte, error)
```
HandleFunc is the function signature expected from all registered functions










## <a name="MultiRegistry">type</a> [MultiRegistry](https://github.com/dapperlabs/flow-go/tree/master/pkg/network/gossip/v1/registry.go?s=738:799#L21)
``` go
type MultiRegistry struct {
    // contains filtered or unexported fields
}

```
MultiRegistry supports combining multiple registries into one
It is suited for scenarios where multiple nodes are running on the same machine and share the
same gossip layer







### <a name="NewMultiRegistry">func</a> [NewMultiRegistry](https://github.com/dapperlabs/flow-go/tree/master/pkg/network/gossip/v1/registry.go?s=1170:1230#L33)
``` go
func NewMultiRegistry(registries ...Registry) *MultiRegistry
```
NewMultiRegistry receives a set of arbitrary number of registers and consolidates them into a MultiRegistry type
Note: If there are registries containing the same msgType name, then one of
them will be overwritten.





### <a name="MultiRegistry.MessageTypes">func</a> (\*MultiRegistry) [MessageTypes](https://github.com/dapperlabs/flow-go/tree/master/pkg/network/gossip/v1/registry.go?s=859:920#L26)
``` go
func (mr *MultiRegistry) MessageTypes() map[string]HandleFunc
```
MessageTypes returns the list of msgTypes to be served




## <a name="Node">type</a> [Node](https://github.com/dapperlabs/flow-go/tree/master/pkg/network/gossip/v1/gnode.go?s=554:971#L26)
``` go
type Node struct {
    // contains filtered or unexported fields
}

```
Node is holding the required information for a functioning async gossip node







### <a name="NewNode">func</a> [NewNode](https://github.com/dapperlabs/flow-go/tree/master/pkg/network/gossip/v1/gnode.go?s=1083:1193#L45)
``` go
func NewNode(msgTypesRegistry Registry, addr string, peers []string, staticFanoutNum int, queueSize int) *Node
```
NewNode returns a new instance of gnode with a specified registry, static fanout set size, and queue size.





### <a name="Node.AsyncGossip">func</a> (\*Node) [AsyncGossip](https://github.com/dapperlabs/flow-go/tree/master/pkg/network/gossip/v1/gnode.go?s=7872:8003#L232)
``` go
func (n *Node) AsyncGossip(ctx context.Context, payload []byte, recipients []string, msgType string) ([]*shared.GossipReply, error)
```
AsyncGossip synchronizes over the delivery
i.e., sends a message to all recipients, and only blocks for delivery without blocking for their response




### <a name="Node.AsyncQueue">func</a> (\*Node) [AsyncQueue](https://github.com/dapperlabs/flow-go/tree/master/pkg/network/gossip/v1/gnode.go?s=9406:9508#L271)
``` go
func (n *Node) AsyncQueue(ctx context.Context, msg *shared.GossipMessage) (*shared.GossipReply, error)
```
AsyncQueue is invoked remotely using the gRPC stub,
it receives a message from a remote node and places it inside the local nodes queue
it is synchronized with the remote node on the message reception (and NOT reply), i.e., blocks the remote node until either
a timeout or placement of the message into the queue




### <a name="Node.RegisterFunc">func</a> (\*Node) [RegisterFunc](https://github.com/dapperlabs/flow-go/tree/master/pkg/network/gossip/v1/gnode.go?s=13616:13679#L423)
``` go
func (n *Node) RegisterFunc(msgType string, f HandleFunc) error
```
RegisterFunc allows the addition of new message types to the node's registry




### <a name="Node.Serve">func</a> (\*Node) [Serve](https://github.com/dapperlabs/flow-go/tree/master/pkg/network/gossip/v1/gnode.go?s=12665:12714#L379)
``` go
func (n *Node) Serve(listener net.Listener) error
```
Serve starts an async node grpc server, and its sweeper as well




### <a name="Node.SyncGossip">func</a> (\*Node) [SyncGossip](https://github.com/dapperlabs/flow-go/tree/master/pkg/network/gossip/v1/gnode.go?s=5736:5866#L175)
``` go
func (n *Node) SyncGossip(ctx context.Context, payload []byte, recipients []string, msgType string) ([]*shared.GossipReply, error)
```
SyncGossip synchronizes over the reply of recipients
i.e., it sends a message to all recipients and blocks for their reply




### <a name="Node.SyncQueue">func</a> (\*Node) [SyncQueue](https://github.com/dapperlabs/flow-go/tree/master/pkg/network/gossip/v1/gnode.go?s=10885:10986#L316)
``` go
func (n *Node) SyncQueue(ctx context.Context, msg *shared.GossipMessage) (*shared.GossipReply, error)
```
SyncQueue is invoked remotely using the gRPC stub,
it receives a message from a remote node and places it inside the local nodes queue
it is synchronized with the remote node on the message reply, i.e., blocks the remote node until either
a timeout or a reply is getting prepared




## <a name="Registry">type</a> [Registry](https://github.com/dapperlabs/flow-go/tree/master/pkg/network/gossip/v1/registry.go?s=488:553#L14)
``` go
type Registry interface {
    MessageTypes() map[string]HandleFunc
}
```
Registry supplies the msgTypes to be called by Gossip Messages
We assume each registry to enclose the set of functions of a single type of node e.g., execution node














- - -
Generated by [godoc2md](http://godoc.org/github.com/lanre-ade/godoc2md)
