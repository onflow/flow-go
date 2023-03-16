package wintermute

import (
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/insecure"
	"github.com/onflow/flow-go/insecure/wintermute"
	"github.com/onflow/flow-go/integration/testnet"
	"github.com/onflow/flow-go/integration/tests/bft"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

// Suite represents a test suite evaluating the integration of the testnet against
// happy path of Corrupted Conduit Framework (CCF) for BFT testing.
type Suite struct {
	bft.BaseSuite

	// execution nodes: 2 corrupted, one honest
	corruptedEN1Id flow.Identifier // corrupted execution node 1
	corruptedEN2Id flow.Identifier // corrupted execution node 2
	honestENId     flow.Identifier // honest execution node

	// verification nodes: 3 corrupted, one honest
	corruptedVnIds flow.IdentifierList
	honestVN       flow.Identifier // honest verification node

	Orchestrator *wintermute.Orchestrator
}

// SetupSuite runs a bare minimum Flow network to function correctly with the following roles:
// - Two collector nodes.
// - Four consensus nodes.
// - Three execution nodes (two corrupted).
// - Four verification nodes (three corrupted).
// - One ghost node (as an execution node).
//
// Moreover, chunk alpha is set to 3, meaning each chunk is assigned to three-out-of-four verification nodes.
// However, each chunk is sealed with two approvals.
// This guarantees that each chunk is assigned to at least two corrupted verification nodes, and they are
// enough to approve and seal the chunk.
func (s *Suite) SetupSuite() {
	s.BaseSuite.SetupSuite()
	// filter out base suite execution, verification and access nodes
	s.NodeConfigs = s.NodeConfigs.Filter(func(n testnet.NodeConfig) bool {
		if n.Ghost {
			return true
		}
		return n.Role != flow.RoleExecution && n.Role != flow.RoleVerification && n.Role != flow.RoleConsensus
	})

	chunkAlpha := "--chunk-alpha=3" // each chunk is assigned to 3 VNs.
	// generate the four consensus identities
	for _, nodeID := range unittest.IdentifierListFixture(4) {
		nodeConfig := testnet.NewNodeConfig(flow.RoleConsensus,
			testnet.WithID(nodeID),
			testnet.WithLogLevel(zerolog.ErrorLevel),
			testnet.WithAdditionalFlag(chunkAlpha),

			// two approvals needed to seal a chunk
			testnet.WithAdditionalFlag("--required-verification-seal-approvals=2"),
			testnet.WithAdditionalFlag("--required-construction-seal-approvals=2"),
		)
		s.NodeConfigs = append(s.NodeConfigs, nodeConfig)
	}

	// generates four verification nodes: three corrupted, one honest
	s.corruptedVnIds = unittest.IdentifierListFixture(3)
	for _, nodeID := range s.corruptedVnIds {
		verConfig := testnet.NewNodeConfig(flow.RoleVerification,
			testnet.WithID(nodeID),
			testnet.WithAdditionalFlag(chunkAlpha),
			testnet.WithLogLevel(zerolog.FatalLevel),
			testnet.AsCorrupted())
		s.NodeConfigs = append(s.NodeConfigs, verConfig)
	}

	// honest verification node
	s.honestVN = unittest.IdentifierFixture()
	ver4Config := testnet.NewNodeConfig(flow.RoleVerification,
		testnet.WithAdditionalFlag(chunkAlpha),
		testnet.WithID(s.honestVN),
		testnet.WithLogLevel(zerolog.FatalLevel))
	s.NodeConfigs = append(s.NodeConfigs, ver4Config)

	// generates three execution nodes: two corrupted and one honest
	// corrupted EN1
	s.corruptedEN1Id = unittest.IdentifierFixture()
	exe1Config := testnet.NewNodeConfig(flow.RoleExecution,
		testnet.WithID(s.corruptedEN1Id),
		testnet.WithLogLevel(zerolog.ErrorLevel),
		testnet.AsCorrupted())
	s.NodeConfigs = append(s.NodeConfigs, exe1Config)

	// corrupted EN2
	s.corruptedEN2Id = unittest.IdentifierFixture()
	exe2Config := testnet.NewNodeConfig(flow.RoleExecution,
		testnet.WithID(s.corruptedEN2Id),
		testnet.WithLogLevel(zerolog.ErrorLevel),
		testnet.AsCorrupted())
	s.NodeConfigs = append(s.NodeConfigs, exe2Config)

	// honest EN
	s.honestENId = unittest.IdentifierFixture()
	exe3Config := testnet.NewNodeConfig(flow.RoleExecution,
		testnet.WithID(s.honestENId),
		testnet.WithLogLevel(zerolog.ErrorLevel))
	s.NodeConfigs = append(s.NodeConfigs, exe3Config)

	s.BaseSuite.StartCorruptedNetwork(
		"wintermute_tests",
		10_000,
		100_000,
		func() insecure.AttackOrchestrator {
			s.Orchestrator = wintermute.NewOrchestrator(s.Log, s.Net.CorruptedIdentities().NodeIDs(), s.Net.Identities())
			return s.Orchestrator
		},
	)
}
