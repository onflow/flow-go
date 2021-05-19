package test

import (
	"testing"

	"github.com/onflow/flow-go/engine/testutil"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/network/stub"
	"github.com/onflow/flow-go/utils/unittest"
)

// Wrap several protocol.State modules in one to enable epoch builder
// Use Signer from hotstuff integration tests
func TestClusterSwitchover(t *testing.T) {
	unittest.LogVerbose()

	identity := unittest.IdentityFixture(unittest.WithRole(flow.RoleCollection))
	participants := unittest.CompleteIdentitySet(identity)
	rootSnapshot := unittest.RootSnapshotFixture(participants)
	hub := stub.NewNetworkHub()

	node := testutil.CollectionNode(t, hub, identity, rootSnapshot)

	<-node.Ready()
	<-node.Done()
}
