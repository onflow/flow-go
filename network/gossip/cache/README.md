# cache
`import "github.com/dapperlabs/flow-go/network/gossip/cache"`

* [Overview](#pkg-overview)
* [Index](#pkg-index)

## <a name="pkg-overview">Overview</a>
The cache package implements the struct MemHashCache, which is an implementation of the [HashCache](https://github.com/dapperlabs/flow-go/blob/Gossip_Team-Yahya/Development/network/gossip/hashcache.go) interface used in Gossip. The cache is used to track the hashes of received messages in order to not receive identical messages more than once.


## <a name="pkg-index">Index</a>
* [type MemHashCache](#MemHashCache)
  * [func NewMemHashCache() *MemHashCache](#NewMemHashCache)
  * [func (mhc *MemHashCache) Confirm(hash string) bool](#MemHashCache.Confirm)
  * [func (mhc *MemHashCache) IsConfirmed(hash string) bool](#MemHashCache.IsConfirmed)
  * [func (mhc *MemHashCache) IsReceived(hash string) bool](#MemHashCache.IsReceived)
  * [func (mhc *MemHashCache) Receive(hash string) bool](#MemHashCache.Receive)


#### <a name="pkg-files">Package files</a>
[cache.go](https://github.com/dapperlabs/flow-go/tree/master/network/gossip/cache/cache.go)






## <a name="MemHashCache">type</a> [MemHashCache](https://github.com/dapperlabs/flow-go/tree/master/network/gossip/cache/cache.go?s=224:284#L11)
``` go
type MemHashCache struct {
    // contains filtered or unexported fields
}

```
MemHashCache implements an on-memory cache of hashes.
Note: as both underlying fields of memoryHashCache are thread-safe, the memoryHashCache itself does
not require a mutex lock







### <a name="NewMemHashCache">func</a> [NewMemHashCache](https://github.com/dapperlabs/flow-go/tree/master/network/gossip/cache/cache.go?s=329:365#L17)
``` go
func NewMemHashCache() *MemHashCache
```
NewMemHashCache returns an empty cache7





### <a name="MemHashCache.Confirm">func</a> (\*MemHashCache) [Confirm](https://github.com/dapperlabs/flow-go/tree/master/network/gossip/cache/cache.go?s=834:884#L36)
``` go
func (mhc *MemHashCache) Confirm(hash string) bool
```
Confirm takes a hash and sets it as confirmed
hash: the hash to be checked




### <a name="MemHashCache.IsConfirmed">func</a> (\*MemHashCache) [IsConfirmed](https://github.com/dapperlabs/flow-go/tree/master/network/gossip/cache/cache.go?s=1014:1068#L42)
``` go
func (mhc *MemHashCache) IsConfirmed(hash string) bool
```
IsConfirmed takes a hash and checks if this hash is confirmed
hash: the hash to be checked




### <a name="MemHashCache.IsReceived">func</a> (\*MemHashCache) [IsReceived](https://github.com/dapperlabs/flow-go/tree/master/network/gossip/cache/cache.go?s=663:716#L30)
``` go
func (mhc *MemHashCache) IsReceived(hash string) bool
```
IsReceived takes a hash and checks if this hash is received




### <a name="MemHashCache.Receive">func</a> (\*MemHashCache) [Receive](https://github.com/dapperlabs/flow-go/tree/master/network/gossip/cache/cache.go?s=518:568#L25)
``` go
func (mhc *MemHashCache) Receive(hash string) bool
```
Receive takes a hash and sets it as received in the memory hash cache
