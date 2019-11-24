# util
`import "github.com/dapperlabs/flow-go/network/gossip/util"`

* [Overview](#pkg-overview)
* [Index](#pkg-index)

## <a name="pkg-overview">Overview</a>
The util package contains a variety of functions that are widely used within the gossip layer. It contains 3 different files:
* helper.go: Contains helper functions that are used throughout the gossip layer
* random_selector.go: Contains a random generator that generates a unique number from a given range each time it is used
* socket.go: Provides helper functions that allow the user to use Sockets, which are representations of IP addresses and ports which can be compactly serialized into bytes. 



## <a name="pkg-index">Index</a>
* [func ComputeHash(msg *messages.GossipMessage) ([]byte, error)](#ComputeHash)
* [func ExtractHashMsgInfo(hashMsgBytes []byte) ([]byte, *messages.Socket, error)](#ExtractHashMsgInfo)
* [func NewSocket(address string) (*messages.Socket, error)](#NewSocket)
* [func RandomSubSet(list []string, size int) ([]string, error)](#RandomSubSet)
* [func SocketToString(s *messages.Socket) string](#SocketToString)


#### <a name="pkg-files">Package files</a>
[helper.go](https://github.com/dapperlabs/flow-go/tree/master/network/gossip/util/helper.go) [random_selector.go](https://github.com/dapperlabs/flow-go/tree/master/network/gossip/util/random_selector.go) [socket.go](https://github.com/dapperlabs/flow-go/tree/master/network/gossip/util/socket.go)





## <a name="ComputeHash">func</a> [ComputeHash](https://github.com/dapperlabs/flow-go/tree/master/network/gossip/util/helper.go?s=1339:1400#L52)
``` go
func ComputeHash(msg *messages.GossipMessage) ([]byte, error)
```
computeHash computes the hash of GossipMessage using sha256 algorithm



## <a name="ExtractHashMsgInfo">func</a> [ExtractHashMsgInfo](https://github.com/dapperlabs/flow-go/tree/master/network/gossip/util/helper.go?s=277:355#L13)
``` go
func ExtractHashMsgInfo(hashMsgBytes []byte) ([]byte, *messages.Socket, error)
```
extractHashMsgInfo receives the bytes of a HashMessage instance,
unmarshals it and returns its components



## <a name="NewSocket">func</a> [NewSocket](https://github.com/dapperlabs/flow-go/tree/master/network/gossip/util/socket.go?s=439:495#L17)
``` go
func NewSocket(address string) (*messages.Socket, error)
```
newSocket takes an IP address and a port and returns a socket that encapsulates this information



## <a name="RandomSubSet">func</a> [RandomSubSet](https://github.com/dapperlabs/flow-go/tree/master/network/gossip/util/helper.go?s=647:707#L23)
``` go
func RandomSubSet(list []string, size int) ([]string, error)
```
Pick a random subset of a list



## <a name="SocketToString">func</a> [SocketToString](https://github.com/dapperlabs/flow-go/tree/master/network/gossip/util/socket.go?s=1496:1542#L59)
``` go
func SocketToString(s *messages.Socket) string
```
SocketToString extracts an address from a given socket
TODO: IPV6 support
