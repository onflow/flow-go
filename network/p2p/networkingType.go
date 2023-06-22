package p2p

// NetworkingType is the type of the Flow networking layer. It is used to differentiate between the public (i.e., unstaked)
// and private (i.e., staked) networks.
type NetworkingType uint8

const (
	// PrivateNetwork indicates that the staked private-side of the Flow blockchain that nodes can only join and leave
	// with a staking requirement.
	PrivateNetwork NetworkingType = iota + 1
	// PublicNetwork indicates that the unstaked public-side of the Flow blockchain that nodes can join and leave at will
	// with no staking requirement.
	PublicNetwork
)
