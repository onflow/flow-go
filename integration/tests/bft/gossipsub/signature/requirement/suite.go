package requirement

import (
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/insecure"
	"github.com/onflow/flow-go/integration/testnet"
	"github.com/onflow/flow-go/integration/tests/bft"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

// Suite represents a test suite ensures libP2P signature verification works as expected.
type Suite struct {
	bft.BaseSuite
	attackerVNIDNoSigning   flow.Identifier // corrupt attacker EN id, this node has message signing disabled
	attackerVNIDWithSigning flow.Identifier // corrupt attacker EN id, this node has message signing enabled
	victimENID              flow.Identifier // corrupt attacker VN id
	Orchestrator            *Orchestrator
}

// SetupSuite runs a bare minimum Flow network to function correctly along with 2 attacker nodes and 1 victim node.
// - Corrupt VN with message signing disabled.
// - Corrupt VN with message signing enabled.
// - Corrupt EN that will serve as the victim node.
func (s *Suite) SetupSuite() {
	s.BaseSuite.SetupSuite()
	// filter out base suite execution and verification nodes
	s.NodeConfigs = s.NodeConfigs.Filter(func(n testnet.NodeConfig) bool {
		if n.Ghost {
			return true
		}
		return n.Role != flow.RoleExecution && n.Role != flow.RoleVerification
	})

	// generate 2 corrupt verification nodes
	s.attackerVNIDNoSigning = unittest.IdentifierFixture()
	s.NodeConfigs = append(s.NodeConfigs, testnet.NewNodeConfig(flow.RoleVerification,
		testnet.WithID(s.attackerVNIDNoSigning),
		testnet.WithLogLevel(zerolog.FatalLevel),
		// turn off message signing and signature verification using corrupt builder flags
		testnet.WithAdditionalFlag("--pubsub-message-signing=false"),
		testnet.WithAdditionalFlag("--pubsub-strict-sig-verification=false"),
		testnet.AsCorrupted()))

	s.attackerVNIDWithSigning = unittest.IdentifierFixture()
	s.NodeConfigs = append(s.NodeConfigs, testnet.NewNodeConfig(flow.RoleVerification,
		testnet.WithID(s.attackerVNIDWithSigning),
		testnet.WithLogLevel(zerolog.FatalLevel),
		testnet.AsCorrupted()))

	// generate 1 corrupt execution node
	s.victimENID = unittest.IdentifierFixture()
	s.NodeConfigs = append(s.NodeConfigs, testnet.NewNodeConfig(flow.RoleExecution,
		testnet.WithID(s.victimENID),
		testnet.WithLogLevel(zerolog.FatalLevel),
		testnet.AsCorrupted()))

	s.NodeConfigs = append(s.NodeConfigs, testnet.NewNodeConfig(flow.RoleExecution, testnet.WithLogLevel(zerolog.FatalLevel)))

	s.BaseSuite.StartCorruptedNetwork(
		"bft_signature_validation_test",
		10_000,
		100_000,
		func() insecure.AttackOrchestrator {
			s.Orchestrator = NewOrchestrator(s.T(), s.Log, s.attackerVNIDNoSigning, s.attackerVNIDWithSigning, s.victimENID)
			return s.Orchestrator
		},
	)
}
