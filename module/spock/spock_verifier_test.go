package spock

import (
	"fmt"
	"testing"

	"github.com/onflow/flow-go/model/flow"
	realproto "github.com/onflow/flow-go/state/protocol"
	protocol "github.com/onflow/flow-go/state/protocol/mock"
	"github.com/onflow/flow-go/utils/unittest"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
)

// SpockVerifierTestSuite contains tests against methods of the SpockVerifier scheme
type SpockVerifierTestSuite struct {
	suite.Suite

	// map to hold test idenitities
	identities map[flow.Identifier]*flow.Identity

	snapshot *protocol.Snapshot
	state    *protocol.State

	verifier *Verifier
}

// TestSpockVerifer invokes all the tests in this test suite
func TestSpockVerifer(t *testing.T) {
	suite.Run(t, new(SpockVerifierTestSuite))
}

// Setup test with n verification nodes
func (vs *SpockVerifierTestSuite) SetupTest() {
	exe := unittest.IdentityFixture(unittest.WithRole(flow.RoleExecution))
	ver := unittest.IdentityFixture(unittest.WithRole(flow.RoleVerification))

	// add to identities map
	vs.identities = make(map[flow.Identifier]*flow.Identity)
	vs.identities[exe.NodeID] = exe
	vs.identities[ver.NodeID] = ver

	vs.snapshot.On("Identity", mock.Anything).Return(
		func(nodeID flow.Identifier) *flow.Identity {
			identity := vs.identities[nodeID]
			return identity
		},
		func(nodeID flow.Identifier) error {
			_, found := vs.identities[nodeID]
			if !found {
				return fmt.Errorf("could not get identity (%x)", nodeID)
			}
			return nil
		},
	)

	vs.state = &protocol.State{}
	vs.state.On("AtBlockID", mock.Anything).Return(
		func(blockID flow.Identifier) realproto.Snapshot {
			return vs.snapshot
		},
		nil,
	)

	vs.verifier = NewVerifier(vs.state)
}
