package topicvalidator

import (
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/insecure"
	"github.com/onflow/flow-go/integration/testnet"
	"github.com/onflow/flow-go/integration/tests/bft"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

// Suite represents a test suite evaluating the correctness of different p2p topic validator
// validation conditions.
type Suite struct {
	bft.BaseSuite
	attackerANID flow.Identifier // corrupt attacker AN id
	attackerENID flow.Identifier // corrupt attacker EN id
	victimENID   flow.Identifier // corrupt victim EN id
	victimVNID   flow.Identifier // corrupt victim VN id
	Orchestrator *Orchestrator
}

// SetupSuite runs a bare minimum Flow network to function correctly along with 2 attacker nodes
// and 2 victim nodes.
// - Corrupt AN that will serve as an attacker and send unauthorized messages to a victim EN.
// - Corrupt EN that will serve as an attacker and send unauthorized messages to a victim VN.
// - Corrupt EN with the topic validator enabled that will serve as a victim.
// - Corrupt VN with the topic validator enabled that will serve as a victim.
func (s *Suite) SetupSuite() {
	s.BaseSuite.SetupSuite()

	// filter out base suite execution, verification and access nodes
	s.NodeConfigs = s.NodeConfigs.Filter(func(n testnet.NodeConfig) bool {
		if n.Ghost {
			return true
		}
		return n.Role != flow.RoleExecution && n.Role != flow.RoleVerification && n.Role != flow.RoleAccess
	})

	// create corrupt access node
	s.attackerANID = unittest.IdentifierFixture()
	s.NodeConfigs = append(s.NodeConfigs, testnet.NewNodeConfig(flow.RoleAccess,
		testnet.WithID(s.attackerANID),
		testnet.WithLogLevel(zerolog.FatalLevel),
		testnet.AsCorrupted()))

	// create corrupt verification node with the topic validator enabled. This is the victim
	// node that will be published unauthorized messages from the attacker execution node.
	s.victimVNID = unittest.IdentifierFixture()
	verConfig := testnet.NewNodeConfig(flow.RoleVerification,
		testnet.WithID(s.victimVNID),
		testnet.WithAdditionalFlag("--topic-validator-disabled=false"),
		testnet.WithLogLevel(zerolog.FatalLevel),
		testnet.AsCorrupted())
	s.NodeConfigs = append(s.NodeConfigs, verConfig)

	// generates two execution nodes, 1 of them will be corrupt
	s.attackerENID = unittest.IdentifierFixture()
	exe1Config := testnet.NewNodeConfig(flow.RoleExecution,
		testnet.WithID(s.attackerENID),
		testnet.WithLogLevel(zerolog.FatalLevel),
		testnet.AsCorrupted())
	s.NodeConfigs = append(s.NodeConfigs, exe1Config)

	// create corrupt execution node with the topic validator enabled. This is the victim
	// node that will be published unauthorized messages from the attacker execution node.
	s.victimENID = unittest.IdentifierFixture()
	exe2Config := testnet.NewNodeConfig(flow.RoleExecution,
		testnet.WithID(s.victimENID),
		testnet.WithLogLevel(zerolog.FatalLevel),
		testnet.WithAdditionalFlag("--topic-validator-disabled=false"),
		testnet.AsCorrupted())
	s.NodeConfigs = append(s.NodeConfigs, exe2Config)

	s.BaseSuite.StartCorruptedNetwork(
		"bft_topic_validator_test",
		10_000,
		100_000,
		func() insecure.AttackOrchestrator {
			s.Orchestrator = NewOrchestrator(s.T(), s.Log, s.attackerANID, s.attackerENID, s.victimENID, s.victimVNID)
			return s.Orchestrator
		},
	)
}
