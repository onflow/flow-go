
# registry
`import "github.com/dapperlabs/flow-go/network/gossip/registry"`

* [Overview](#pkg-overview)
* [Index](#pkg-index)

## <a name="pkg-overview">Overview</a>
Package registry provides a (function name, function body) on-memory key-value store with adding and invoking msgTypes functionality.

In the current version of the gossip, Registry is solely utilized internally by the gossip layer for its internal affairs,
and is not meant for the application layer.


## <a name="pkg-index">Index</a>
* [Constants](#pkg-constants)
* [type HandleFunc](#HandleFunc)
* [type MessageType](#MessageType)
* [type Registry](#Registry)
* [type RegistryManager](#RegistryManager)
  * [func NewRegistryManager(registry Registry) *RegistryManager](#NewRegistryManager)
  * [func (r *RegistryManager) AddDefaultTypes(msgType []MessageType, f []HandleFunc) error](#RegistryManager.AddDefaultTypes)
  * [func (r *RegistryManager) AddMessageType(mType *MessageType, f HandleFunc) error](#RegistryManager.AddMessageType)
  * [func (r *RegistryManager) Invoke(ctx context.Context, msgType MessageType, payloadBytes []byte) (*invokeResponse, error)](#RegistryManager.Invoke)


#### <a name="pkg-files">Package files</a>
[registry.go](https://github.com/dapperlabs/flow-go/tree/master/network/gossip/registry/registry.go) [registry_manager.go](https://github.com/dapperlabs/flow-go/tree/master/network/gossip/registry/registry_manager.go)


## <a name="pkg-constants">Constants</a>
``` go
const (
    DefaultTypes = 3
)
```




## <a name="HandleFunc">type</a> [HandleFunc](https://github.com/dapperlabs/flow-go/tree/master/network/gossip/registry/registry.go?s=188:249#L9)
``` go
type HandleFunc func(context.Context, []byte) ([]byte, error)
```
HandleFunc is the function signature expected from all registered functions










## <a name="MessageType">type</a> [MessageType](https://github.com/dapperlabs/flow-go/tree/master/network/gossip/registry/registry.go?s=89:107#L6)
``` go
type MessageType uint64
```
MessageType is a type representing mapping of registry










## <a name="Registry">type</a> [Registry](https://github.com/dapperlabs/flow-go/tree/master/network/gossip/registry/registry.go?s=422:709#L13)
``` go
type Registry interface {

    // MessageTypes returns the mapping of the registry as a map data structure
    // which takes an enum type MessageType that identifies a certain message
    // and the returns a function that is invoked as a response to a message
    MessageTypes() map[MessageType]HandleFunc
}
```
Registry supplies the msgTypes to be called by Gossip Messages
We assume each registry to enclose the set of functions of a single type of node e.g., execution node










## <a name="RegistryManager">type</a> [RegistryManager](https://github.com/dapperlabs/flow-go/tree/master/network/gossip/registry/registry_manager.go?s=490:576#L18)
``` go
type RegistryManager struct {
    // contains filtered or unexported fields
}

```
RegistryManager is used internally to wrap Registries and provide an invocation interface







### <a name="NewRegistryManager">func</a> [NewRegistryManager](https://github.com/dapperlabs/flow-go/tree/master/network/gossip/registry/registry_manager.go?s=661:720#L24)
``` go
func NewRegistryManager(registry Registry) *RegistryManager
```
NewRegistryManager initializes a registry manager which manges a given registry





### <a name="RegistryManager.AddDefaultTypes">func</a> (\*RegistryManager) [AddDefaultTypes](https://github.com/dapperlabs/flow-go/tree/master/network/gossip/registry/registry_manager.go?s=1407:1488#L51)
``` go
func (r *RegistryManager) AddDefaultTypes(msgType []MessageType, f []HandleFunc) error
```
AddDefaultType adds defaults handlers to registry




### <a name="RegistryManager.AddMessageType">func</a> (\*RegistryManager) [AddMessageType](https://github.com/dapperlabs/flow-go/tree/master/network/gossip/registry/registry_manager.go?s=1690:1765#L63)
``` go
func (r *RegistryManager) AddMessageType(mType *MessageType, f HandleFunc) error
```
AddMessageType adds a msgType and its handler to the RegistryManager




### <a name="RegistryManager.Invoke">func</a> (\*RegistryManager) [Invoke](https://github.com/dapperlabs/flow-go/tree/master/network/gossip/registry/registry_manager.go?s=966:1081#L37)
``` go
func (r *RegistryManager) Invoke(ctx context.Context, msgType MessageType, payloadBytes []byte) (*invokeResponse, error)
```
Invoke passes input parameters to given msgType handler in the registry







