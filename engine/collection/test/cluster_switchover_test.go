package test

import (
	"testing"
	"time"

	"github.com/onflow/flow-go/engine/testutil"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/network/stub"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestClusterSwitchover(t *testing.T) {
	unittest.LogVerbose()

	identity := unittest.IdentityFixture(unittest.WithRole(flow.RoleCollection))
	participants := unittest.CompleteIdentitySet(identity)
	rootSnapshot := unittest.RootSnapshotFixture(participants)
	hub := stub.NewNetworkHub()

	node := testutil.CollectionNode(t, hub, identity, rootSnapshot)

	// for now just bring up and down the node
	unittest.RequireCloseBefore(t, node.Ready(), time.Second, "failed to start node")
	unittest.RequireCloseBefore(t, node.Done(), time.Second, "failed to stop node")
}
