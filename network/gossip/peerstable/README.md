# peerstable
`import "github.com/dapperlabs/flow-go/network/gossip/peerstable"`

* [Overview](#pkg-overview)
* [Index](#pkg-index)

## <a name="pkg-overview">Overview</a>
peersTable defines a data structure that keeps track of the mappings from ID to IP and vice versa. It is an implementation of the PeersTable interface defined in [peerstable.go](https://github.com/dapperlabs/flow-go/blob/Gossip_Team-Yahya/Development/network/gossip/peerstable.go)


## <a name="pkg-index">Index</a>
* [type PeersTable](#PeersTable)
  * [func NewPeersTable() (*PeersTable, error)](#NewPeersTable)
  * [func (pt *PeersTable) Add(ID string, IP string)](#PeersTable.Add)
  * [func (pt *PeersTable) GetID(IP string) (string, error)](#PeersTable.GetID)
  * [func (pt *PeersTable) GetIDs(IPs ...string) ([]string, error)](#PeersTable.GetIDs)
  * [func (pt *PeersTable) GetIP(ID string) (string, error)](#PeersTable.GetIP)
  * [func (pt *PeersTable) GetIPs(IDs ...string) ([]string, error)](#PeersTable.GetIPs)


#### <a name="pkg-files">Package files</a>
[peersTable.go](https://github.com/dapperlabs/flow-go/tree/master/network/gossip/peerstable/peersTable.go)






## <a name="PeersTable">type</a> [PeersTable](https://github.com/dapperlabs/flow-go/tree/master/network/gossip/peerstable/peersTable.go?s=163:241#L11)
``` go
type PeersTable struct {
    IDToIP map[string]string
    IPToID map[string]string
}

```
PeersTable is a type that keeps a mapping from IP to an ID and vice versa.







### <a name="NewPeersTable">func</a> [NewPeersTable](https://github.com/dapperlabs/flow-go/tree/master/network/gossip/peerstable/peersTable.go?s=297:338#L17)
``` go
func NewPeersTable() (*PeersTable, error)
```
NewPeersTable returns a new instance of PeersTable





### <a name="PeersTable.Add">func</a> (\*PeersTable) [Add](https://github.com/dapperlabs/flow-go/tree/master/network/gossip/peerstable/peersTable.go?s=488:535#L25)
``` go
func (pt *PeersTable) Add(ID string, IP string)
```
Add adds a new mapping to the peers table




### <a name="PeersTable.GetID">func</a> (\*PeersTable) [GetID](https://github.com/dapperlabs/flow-go/tree/master/network/gossip/peerstable/peersTable.go?s=638:692#L31)
``` go
func (pt *PeersTable) GetID(IP string) (string, error)
```
GetID receives an IP and returns its corresponding ID




### <a name="PeersTable.GetIDs">func</a> (\*PeersTable) [GetIDs](https://github.com/dapperlabs/flow-go/tree/master/network/gossip/peerstable/peersTable.go?s=1478:1539#L68)
``` go
func (pt *PeersTable) GetIDs(IPs ...string) ([]string, error)
```
GetIDs receives a group of IPs and returns their corresponding IDs




### <a name="PeersTable.GetIP">func</a> (\*PeersTable) [GetIP](https://github.com/dapperlabs/flow-go/tree/master/network/gossip/peerstable/peersTable.go?s=881:935#L41)
``` go
func (pt *PeersTable) GetIP(ID string) (string, error)
```
GetIP receives a ID and returns its corresponding IP




### <a name="PeersTable.GetIPs">func</a> (\*PeersTable) [GetIPs](https://github.com/dapperlabs/flow-go/tree/master/network/gossip/peerstable/peersTable.go?s=1139:1200#L52)
``` go
func (pt *PeersTable) GetIPs(IDs ...string) ([]string, error)
```
GetIPs receives a group of IDs and returns their corresponding IPs

