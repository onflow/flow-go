package identity

import "math/big"

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

type NodeRecord struct {
	ID uint
	Address string
	Role NodeRole
	Stake *big.Int
}

type IdentityTable interface {
	Count() int
	Nodes() []NodeIdentity

	GetByID(uint) NodeIdentity
	GetByAddress(string) NodeIdentity

	TotalStake() *big.Int

	FilterByID([]uint) IdentityTable
	FilterByAddress([]string) IdentityTable
	FilterByRole(NodeRole) IdentityTable
	FilterByIndex([]uint) IdentityTable
}

type InMemoryIdentityTable struct {
	nodes []*NodeRecord
	addressMap map[string]*NodeRecord
	idMap map[uint]*NodeRecord
}

func NewInMemoryIdentityTable(nodes []NodeRecord) *InMemoryIdentityTable {
	return &InMemoryIdentityTable{

	}
}