

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
* [type Node](#Node)
  * [func NewNode(config *NodeConfig) *Node](#NewNode)
  * [func (n *Node) AsyncGossip(ctx context.Context, payload []byte, recipients []string, msgType string) ([]*shared.GossipReply, error)](#Node.AsyncGossip)
  * [func (n *Node) AsyncQueue(ctx context.Context, msg *shared.GossipMessage) (*shared.GossipReply, error)](#Node.AsyncQueue)
  * [func (n *Node) RegisterFunc(msgType string, f HandleFunc) error](#Node.RegisterFunc)
  * [func (n *Node) Serve(listener net.Listener)](#Node.Serve)
  * [func (n *Node) SetProtocol(s ServePlacer)](#Node.SetProtocol)
  * [func (n *Node) SyncGossip(ctx context.Context, payload []byte, recipients []string, msgType string) ([]*shared.GossipReply, error)](#Node.SyncGossip)
  * [func (n *Node) SyncQueue(ctx context.Context, msg *shared.GossipMessage) (*shared.GossipReply, error)](#Node.SyncQueue)
* [type NodeConfig](#NodeConfig)
  * [func NewNodeConfig(reg Registry, addr string, peers []string, staticFN int, qSize int) *NodeConfig](#NewNodeConfig)
* [type Registry](#Registry)
* [type ServePlacer](#ServePlacer)


#### <a name="pkg-files">Package files</a>
[cache.go](https://github.com/dapperlabs/flow-go/tree/master/pkg/network/gossip/v1/cache.go) [database.go](https://github.com/dapperlabs/flow-go/tree/master/pkg/network/gossip/v1/database.go) [gerror.go](https://github.com/dapperlabs/flow-go/tree/master/pkg/network/gossip/v1/gerror.go) [gnode.go](https://github.com/dapperlabs/flow-go/tree/master/pkg/network/gossip/v1/gnode.go) [helper.go](https://github.com/dapperlabs/flow-go/tree/master/pkg/network/gossip/v1/helper.go) [message.go](https://github.com/dapperlabs/flow-go/tree/master/pkg/network/gossip/v1/message.go) [network.go](https://github.com/dapperlabs/flow-go/tree/master/pkg/network/gossip/v1/network.go) [node_config.go](https://github.com/dapperlabs/flow-go/tree/master/pkg/network/gossip/v1/node_config.go) [random_selector.go](https://github.com/dapperlabs/flow-go/tree/master/pkg/network/gossip/v1/random_selector.go) [registry.go](https://github.com/dapperlabs/flow-go/tree/master/pkg/network/gossip/v1/registry.go)


## <a name="pkg-constants">Constants</a>
``` go
const DefaultQueueSize = 10
```
DefaultQueueSize is the default size of node queue


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










## <a name="Node">type</a> [Node](https://github.com/dapperlabs/flow-go/tree/master/pkg/network/gossip/v1/gnode.go?s=528:973#L21)
``` go
type Node struct {
    // contains filtered or unexported fields
}

```
Node is holding the required information for a functioning async gossip node







### <a name="NewNode">func</a> [NewNode](https://github.com/dapperlabs/flow-go/tree/master/pkg/network/gossip/v1/gnode.go?s=1152:1190#L43)
``` go
func NewNode(config *NodeConfig) *Node
```
NewNode returns a new instance of a Gossip node with a predefined registry of message types, a set of peers
and a staticFanoutNum indicating the size of the static fanout





### <a name="Node.AsyncGossip">func</a> (\*Node) [AsyncGossip](https://github.com/dapperlabs/flow-go/tree/master/pkg/network/gossip/v1/gnode.go?s=8638:8769#L270)
``` go
func (n *Node) AsyncGossip(ctx context.Context, payload []byte, recipients []string, msgType string) ([]*shared.GossipReply, error)
```
AsyncGossip synchronizes over the delivery
i.e., sends a message to all recipients, and only blocks for delivery without blocking for their response




### <a name="Node.AsyncQueue">func</a> (\*Node) [AsyncQueue](https://github.com/dapperlabs/flow-go/tree/master/pkg/network/gossip/v1/gnode.go?s=10735:10837#L329)
``` go
func (n *Node) AsyncQueue(ctx context.Context, msg *shared.GossipMessage) (*shared.GossipReply, error)
```
AsyncQueue is invoked remotely using the gRPC stub,
it receives a message from a remote node and places it inside the local nodes queue
it is synchronized with the remote node on the message reception (and NOT reply), i.e., blocks the remote node until either
a timeout or placement of the message into the queue




### <a name="Node.RegisterFunc">func</a> (\*Node) [RegisterFunc](https://github.com/dapperlabs/flow-go/tree/master/pkg/network/gossip/v1/gnode.go?s=14501:14564#L450)
``` go
func (n *Node) RegisterFunc(msgType string, f HandleFunc) error
```
RegisterFunc allows the addition of new message types to the node's registry




### <a name="Node.Serve">func</a> (\*Node) [Serve](https://github.com/dapperlabs/flow-go/tree/master/pkg/network/gossip/v1/gnode.go?s=13962:14005#L435)
``` go
func (n *Node) Serve(listener net.Listener)
```
Serve starts an async node grpc server, and its sweeper as well




### <a name="Node.SetProtocol">func</a> (\*Node) [SetProtocol](https://github.com/dapperlabs/flow-go/tree/master/pkg/network/gossip/v1/gnode.go?s=1774:1815#L62)
``` go
func (n *Node) SetProtocol(s ServePlacer)
```
SetProtocol sets the underlying protocol for sending and receiving messages. The protocol should
implement ServePlacer




### <a name="Node.SyncGossip">func</a> (\*Node) [SyncGossip](https://github.com/dapperlabs/flow-go/tree/master/pkg/network/gossip/v1/gnode.go?s=6193:6323#L198)
``` go
func (n *Node) SyncGossip(ctx context.Context, payload []byte, recipients []string, msgType string) ([]*shared.GossipReply, error)
```
SyncGossip synchronizes over the reply of recipients
i.e., it sends a message to all recipients and blocks for their reply




### <a name="Node.SyncQueue">func</a> (\*Node) [SyncQueue](https://github.com/dapperlabs/flow-go/tree/master/pkg/network/gossip/v1/gnode.go?s=12208:12309#L375)
``` go
func (n *Node) SyncQueue(ctx context.Context, msg *shared.GossipMessage) (*shared.GossipReply, error)
```
SyncQueue is invoked remotely using the gRPC stub,
it receives a message from a remote node and places it inside the local nodes queue
it is synchronized with the remote node on the message reply, i.e., blocks the remote node until either
a timeout or a reply is getting prepared




## <a name="NodeConfig">type</a> [NodeConfig](https://github.com/dapperlabs/flow-go/tree/master/pkg/network/gossip/v1/node_config.go?s=722:914#L22)
``` go
type NodeConfig struct {
    // contains filtered or unexported fields
}

```
NodeConfig type is wrapper for the parameters used to construct a gossip node
logger is an instance of the zerolog for printing the log messages by the node
address is the (IP) address of the node itself
peers is the list of all the nodes in the system as IP:port, e.g., localhost:8080
static fanout is the number of gossip partners of the node, a typical number may be 10
queueSize is the buffer size of the node on processing the gossip messages, a typical number may be 10







### <a name="NewNodeConfig">func</a> [NewNodeConfig](https://github.com/dapperlabs/flow-go/tree/master/pkg/network/gossip/v1/node_config.go?s=1291:1389#L36)
``` go
func NewNodeConfig(reg Registry, addr string, peers []string, staticFN int, qSize int) *NodeConfig
```
NewNodeConfig returns a new instance NodeConfig
addr is the (IP) address of the node itself
peers is the list of all the nodes in the system as IP:port, e.g., localhost:8080
static fanout is the number of gossip partners of the node, a typical number may be 10
qSize is the buffer size of the node on processing the gossip messages, a typical number may be 10





## <a name="Registry">type</a> [Registry](https://github.com/dapperlabs/flow-go/tree/master/pkg/network/gossip/v1/registry.go?s=488:586#L14)
``` go
type Registry interface {
    MessageTypes() map[uint64]HandleFunc
    NameMapping() map[string]uint64
}
```
Registry supplies the msgTypes to be called by Gossip Messages
We assume each registry to enclose the set of functions of a single type of node e.g., execution node










## <a name="ServePlacer">type</a> [ServePlacer](https://github.com/dapperlabs/flow-go/tree/master/pkg/network/gossip/v1/network.go?s=275:429#L15)
``` go
type ServePlacer interface {
    Serve(net.Listener)
    Place(context.Context, string, *shared.GossipMessage, bool, gossip.Mode) (*shared.GossipReply, error)
}
```
ServePlacer is an interface for any protocol that can be used as a connection medium for gossip














- - -
Generated by [godoc2md](http://godoc.org/github.com/lanre-ade/godoc2md)
