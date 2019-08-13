// Package identity provides an interface that bundles all relevant information about staked nodes.
// A simple in-memory implementation is included.
package identity

import (
	"fmt"
	"math/big"
	"sort"
	"strconv"
)

type NodeRole string
const (
	CollectorRole NodeRole = "collector"
	ConsensusRole NodeRole = "consensus"
	ExecutorRole NodeRole = "executor"
	VerifierRole NodeRole = "verifier"
	ObserverRole NodeRole = "observer"
)

type NodeIdentity interface {
	ID() uint
	Address() string
	Role() NodeRole
	Stake() *big.Int
	Index() uint
}

type IdentityTable interface {
	Count() int
	Nodes() []NodeIdentity

	GetByID(uint) (NodeIdentity, error)
	GetByAddress(string) (NodeIdentity, error)

	TotalStake() *big.Int

	FilterByID([]uint) IdentityTable
	FilterByAddress([]string) IdentityTable
	FilterByRole(NodeRole) IdentityTable
	FilterByIndex([]uint) IdentityTable
}

// Information about one Node that is independent of potential other nodes that this node is grouped together with.
type NodeRecord struct {
	ID uint
	Address string
	Role NodeRole
	Stake *big.Int
}


// NodeRecords is a slice of *NodeRecord which implements sort.Interface
// Sorting is based solely on NodeRecord.ID
type NodeRecords []*NodeRecord

func (ns NodeRecords) Len() int {
	return len(ns)
}
func (ns NodeRecords) Less(i, j int) bool {
	return ns[i].ID < ns[j].ID
}
func (ns NodeRecords) Swap(i, j int) {
	ns[i], ns[j] = ns[j], ns[i]
}

// Compile-time check that nodeIdentity implements interface NodeIdentity
var _ sort.Interface = (*NodeRecords)(nil)


// Implementation of NodeIdentity interface
type nodeIdentity struct {
	coreID *NodeRecord
	index uint
}

func (i nodeIdentity) ID() uint {
	return i.coreID.ID
}

func (i nodeIdentity) Address() string {
	return i.coreID.Address
}

func (i nodeIdentity) Role() NodeRole {
	return i.coreID.Role
}

func (i nodeIdentity) Stake() *big.Int {
	return i.coreID.Stake
}

func (i nodeIdentity) Index() uint {
	return i.index
}

// Compile-time check that nodeIdentity implements interface NodeIdentity
var _ NodeIdentity = (*nodeIdentity)(nil)



// In-memory Implementation of the interface IdentityTable
type InMemoryIdentityTable struct {
	nodes []*nodeIdentity
	addressMap map[string]*nodeIdentity
	idMap map[uint]*nodeIdentity

}

func (t InMemoryIdentityTable) Count() int {
	return len(t.nodes)
}

func (t InMemoryIdentityTable) Nodes() []NodeIdentity {
	identities := make([]NodeIdentity, len(t.nodes))
	for i, n := range t.nodes {
		identities[i] = n
	}
	return identities
}

func (t InMemoryIdentityTable) GetByID(id uint) (NodeIdentity, error) {
	value, found := t.idMap[id];
	if !found {
		return nil, &NodeNotFoundError{fmt.Sprint(id)}
	}
	return value, nil
}

func (t InMemoryIdentityTable) GetByAddress(address string) (NodeIdentity, error) {
	value, found := t.addressMap[address];
	if !found {
		return nil, &NodeNotFoundError{address}
	}
	return value, nil
}

func (t InMemoryIdentityTable) TotalStake() *big.Int {
	s := big.NewInt(0)
	for _, n := range t.nodes {
		s.Add(s, n.Stake())
	}
	return s
}

func (t InMemoryIdentityTable) FilterByID(ids []uint) IdentityTable {
	nodes := make([]*NodeRecord, len(ids))
	var n *nodeIdentity
	var found bool
	var idx int = 0
	for _, id := range ids {
		n, found = t.idMap[id]
		if found {
			nodes[idx] = n.coreID
			idx += 1
		}
	}
	return NewInMemoryIdentityTable(nodes[0:idx])
}

func (t InMemoryIdentityTable) FilterByAddress(addresses []string) IdentityTable {
	nodes := make([]*NodeRecord, len(addresses))
	var n *nodeIdentity
	var found bool
	var idx int = 0
	for _, addr := range addresses {
		n, found = t.addressMap[addr]
		if found {
			nodes[idx] = n.coreID
			idx += 1
		}
	}
	return NewInMemoryIdentityTable(nodes[0:idx])
}

func (t InMemoryIdentityTable) FilterByRole(role NodeRole) IdentityTable {
	panic("implement me")
}

func (t InMemoryIdentityTable) FilterByIndex([]uint) IdentityTable {
	panic("implement me")
}

type NodeNotFoundError struct {
	key string
}

func (e *NodeNotFoundError) Error() string {
	return fmt.Sprintf("node with '%s' not found", e.key)
}






func NewInMemoryIdentityTable(nodes []*NodeRecord) *InMemoryIdentityTable {


	// While the slice `nodes` is copied, the data in the slice is not sufficient to sort the slice without mutating the underlying array
	// For more details, see https://blog.golang.org/go-slices-usage-and-internals

	nds := make([]*NodeRecord, len(nodes))

	//sort.Sort(ByAge(family))
	nds := make([]*NodeRecord, len(nodes))
	copy(nds, nodes)
	sort.Sort(NodeRecords(nds))

	nidentities := make([]*nodeIdentity, len(nodes))
	addressMap := make(map[string]*nodeIdentity)
	idMap := make(map[uint]*nodeIdentity)

	for i, r := range nodes {
		nodeId := &nodeIdentity{
			coreID: r,
			index: uint(i),
		}
		nidentities[i] = nodeId
		addressMap[r.Address] = nodeId
		idMap[r.ID] = nodeId
	}

	return &InMemoryIdentityTable{
		nidentities,
		addressMap,
		idMap,
	}
}


