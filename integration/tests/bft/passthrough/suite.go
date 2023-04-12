package passthrough

import (
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/insecure"
	"github.com/onflow/flow-go/integration/testnet"
	"github.com/onflow/flow-go/integration/tests/bft"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

const (
	// corruptENs is the number of corrupt execution nodes to create
	corruptENs int = 2

	// corruptVNs is the number of corrupt verification nodes to create
	corruptVNs int = 2
)

// Suite represents a test suite evaluating the integration of the testnet against
// happy path of Corrupted Conduit Framework (CCF) for BFT testing.
type Suite struct {
	bft.BaseSuite
	exe1ID       flow.Identifier     // corrupted execution node 1
	exe2ID       flow.Identifier     // corrupted execution node 2
	exeIDs       flow.IdentifierList // corrupt execution nodes
	verIDs       flow.IdentifierList // corrupt verification nodes list
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

	// generate corrupt verification nodes
	s.verIDs = make([]flow.Identifier, corruptVNs)
	for i := range s.verIDs {
		s.verIDs[i] = unittest.IdentifierFixture()
		verConfig := testnet.NewNodeConfig(flow.RoleVerification,
			testnet.WithID(s.verIDs[i]),
			testnet.WithLogLevel(zerolog.InfoLevel),
			testnet.AsCorrupted())
		s.NodeConfigs = append(s.NodeConfigs, verConfig)
	}

	// generate corrupt execution nodes
	s.exeIDs = make([]flow.Identifier, corruptENs)
	for i := range s.exeIDs {
		s.exeIDs[i] = unittest.IdentifierFixture()
		exeConfig := testnet.NewNodeConfig(flow.RoleExecution,
			testnet.WithID(s.exeIDs[i]),
			testnet.WithLogLevel(zerolog.ErrorLevel),
			testnet.AsCorrupted())
		s.NodeConfigs = append(s.NodeConfigs, exeConfig)
	}

	s.exe1ID = s.exeIDs[0]
	s.exe2ID = s.exeIDs[1]

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
