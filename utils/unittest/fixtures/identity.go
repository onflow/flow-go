package fixtures

import (
	"github.com/onflow/crypto"

	"github.com/onflow/flow-go/model/flow"
)

// Identity is the default options factory for [flow.Identity] generation.
var Identity identityFactory

type identityFactory struct{}

type IdentityOption func(*IdentityGenerator, *flow.Identity)

// WithNodeID is an option that sets the `NodeID` of the identity.
func (f identityFactory) WithNodeID(nodeID flow.Identifier) IdentityOption {
	return func(g *IdentityGenerator, identity *flow.Identity) {
		identity.NodeID = nodeID
	}
}

// WithRole is an option that sets the `Role` of the identity.
func (f identityFactory) WithRole(role flow.Role) IdentityOption {
	return func(g *IdentityGenerator, identity *flow.Identity) {
		identity.Role = role
	}
}

// WithAddress is an option that sets the `Address` of the identity.
func (f identityFactory) WithAddress(address string) IdentityOption {
	return func(g *IdentityGenerator, identity *flow.Identity) {
		identity.Address = address
	}
}

// WithInitialWeight is an option that sets the `InitialWeight` of the identity.
func (f identityFactory) WithInitialWeight(weight uint64) IdentityOption {
	return func(g *IdentityGenerator, identity *flow.Identity) {
		identity.InitialWeight = weight
	}
}

// WithStakingPubKey is an option that sets the `StakingPubKey` of the identity.
func (f identityFactory) WithStakingPubKey(key crypto.PublicKey) IdentityOption {
	return func(g *IdentityGenerator, identity *flow.Identity) {
		identity.StakingPubKey = key
	}
}

// WithNetworkPubKey is an option that sets the `NetworkPubKey` of the identity.
func (f identityFactory) WithNetworkPubKey(key crypto.PublicKey) IdentityOption {
	return func(g *IdentityGenerator, identity *flow.Identity) {
		identity.NetworkPubKey = key
	}
}

// WithEpochParticipationStatus is an option that sets the `EpochParticipationStatus` of the identity.
func (f identityFactory) WithEpochParticipationStatus(status flow.EpochParticipationStatus) IdentityOption {
	return func(g *IdentityGenerator, identity *flow.Identity) {
		identity.EpochParticipationStatus = status
	}
}

// WithAllRoles is an option that sets the `Role` of the identity. When used with `List()`, it will
// set successive identities to different roles, cycling through all roles.
func (f identityFactory) WithAllRoles() IdentityOption {
	return f.WithAllRolesExcept()
}

// WithAllRolesExcept is an option that sets the `Role` of the identity. When used with `List()`, it will
// set successive identities to different roles, cycling through all roles except the ones specified.
func (f identityFactory) WithAllRolesExcept(except ...flow.Role) IdentityOption {
	omitRoles := make(map[flow.Role]struct{})
	for _, role := range except {
		omitRoles[role] = struct{}{}
	}

	// create a set of roles without the omitted roles
	roles := make(flow.RoleList, 0)
	for _, role := range flow.Roles() {
		if _, ok := omitRoles[role]; !ok {
			roles = append(roles, role)
		}
	}

	i := 0
	return func(g *IdentityGenerator, identity *flow.Identity) {
		identity.Role = roles[i%len(roles)]
		i++
	}
}

// IdentityGenerator generates identities with consistent randomness.
type IdentityGenerator struct {
	identityFactory

	random      *RandomGenerator
	cryptoGen   *CryptoGenerator
	identifiers *IdentifierGenerator
	addresses   *AddressGenerator
}

// NewIdentityGenerator creates a new IdentityGenerator.
func NewIdentityGenerator(
	random *RandomGenerator,
	cryptoGen *CryptoGenerator,
	identifiers *IdentifierGenerator,
	addresses *AddressGenerator,
) *IdentityGenerator {
	return &IdentityGenerator{
		random:      random,
		cryptoGen:   cryptoGen,
		identifiers: identifiers,
		addresses:   addresses,
	}
}

// Fixture generates a [flow.Identity] with random data based on the provided options.
func (g *IdentityGenerator) Fixture(opts ...IdentityOption) *flow.Identity {
	statuses := []flow.EpochParticipationStatus{
		flow.EpochParticipationStatusActive,
		flow.EpochParticipationStatusLeaving,
		flow.EpochParticipationStatusJoining,
		// omit ejected status
	}

	// 50% chance of being 1000 weight
	weight := uint64(1000)
	if g.random.Bool() {
		weight = g.random.Uint64InRange(1, 999)
	}

	identity := &flow.Identity{
		IdentitySkeleton: flow.IdentitySkeleton{
			NodeID:        g.identifiers.Fixture(),
			Address:       g.addresses.Fixture().String(),
			Role:          RandomElement(g.random, flow.Roles()),
			InitialWeight: weight,
			StakingPubKey: g.cryptoGen.StakingPrivateKey().PublicKey(),
			NetworkPubKey: g.cryptoGen.NetworkingPrivateKey().PublicKey(),
		},
		DynamicIdentity: flow.DynamicIdentity{
			EpochParticipationStatus: RandomElement(g.random, statuses),
		},
	}

	for _, opt := range opts {
		opt(g, identity)
	}

	return identity
}

// List generates a list of [flow.Identity].
func (g *IdentityGenerator) List(n int, opts ...IdentityOption) flow.IdentityList {
	identities := make(flow.IdentityList, n)
	for i := range n {
		identities[i] = g.Fixture(opts...)
	}
	return identities
}
