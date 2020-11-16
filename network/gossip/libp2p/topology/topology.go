package topology

import "github.com/onflow/flow-go/model/flow"

// Topology provides a subset of nodes which a given node should directly connect to for 1-k messaging
type Topology interface {
	// SubsetChannel returns a random subset of the identity list that is passed. `shouldHave` represents set of
	// identities that should be included in the returned subset.
	// Returned identities should all subscribed to the specified `channel`.
	// Note: this method should not include identity of its executor.
	SubsetChannel(ids flow.IdentityList, shouldHave flow.IdentityList, channel string) (flow.IdentityList, error)
	// SubsetRole returns a random subset of the identity list that is passed. `shouldHave` represents set of
	// identities that should be included in the returned subset.
	// Returned identities should all be of one of the specified `roles`.
	// Note: this method should not include identity of its executor.
	SubsetRole(ids flow.IdentityList, shouldHave flow.IdentityList, roles flow.RoleList) (flow.IdentityList, error)
}
