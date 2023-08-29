package framework

import (
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/insecure"
	"github.com/onflow/flow-go/integration/testnet"
	"github.com/onflow/flow-go/integration/tests/bft"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

// Suite represents a test suite evaluating the integration of the testnet against
// happy path of Corrupted Conduit Framework (CCF) for BFT testing.
type Suite struct {
	bft.BaseSuite
	exe1ID       flow.Identifier // corrupted execution node 1
	exe2ID       flow.Identifier // corrupted execution node 2
	verID        flow.Identifier // corrupted verification node
	Orchestrator *orchestrator
}

// SetupSuite runs a bare minimum Flow network to function correctly with the following corrupted nodes:
// - two corrupted execution node
// - One corrupted verification node
func (s *Suite) SetupSuite() {
	s.BaseSuite.SetupSuite()

	// filter out base suite execution and verification nodes
	s.NodeConfigs = s.NodeConfigs.Filter(func(n testnet.NodeConfig) bool {
		if n.Ghost {
			return true
		}
		return n.Role != flow.RoleExecution && n.Role != flow.RoleVerification
	})

	// generates one corrupted verification node
	s.verID = unittest.IdentifierFixture()
	verConfig := testnet.NewNodeConfig(flow.RoleVerification,
		testnet.WithID(s.verID),
		testnet.WithLogLevel(zerolog.ErrorLevel),
		testnet.AsCorrupted())
	s.NodeConfigs = append(s.NodeConfigs, verConfig)

	// generates two corrupted execution nodes
	s.exe1ID = unittest.IdentifierFixture()
	exe1Config := testnet.NewNodeConfig(flow.RoleExecution,
		testnet.WithID(s.exe1ID),
		testnet.WithLogLevel(zerolog.ErrorLevel),
		testnet.AsCorrupted())
	s.NodeConfigs = append(s.NodeConfigs, exe1Config)

	s.exe2ID = unittest.IdentifierFixture()
	exe2Config := testnet.NewNodeConfig(flow.RoleExecution,
		testnet.WithID(s.exe2ID),
		testnet.WithLogLevel(zerolog.ErrorLevel),
		testnet.AsCorrupted())
	s.NodeConfigs = append(s.NodeConfigs, exe2Config)

	s.BaseSuite.StartCorruptedNetwork(
		"bft_passthrough_test",
		10_000,
		100_000,
		func() insecure.AttackOrchestrator {
			s.Orchestrator = NewDummyOrchestrator(s.T(), s.Log)
			return s.Orchestrator
		},
	)
}
