package spam

import (
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/insecure"
	"github.com/onflow/flow-go/integration/testnet"
	"github.com/onflow/flow-go/integration/tests/bft"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

type Suite struct {
	bft.BaseSuite
	orchestrator *orchestrator
}

func (s *Suite) SetupSuite() {
	s.BaseSuite.SetupSuite()

	// create spam victim VN node
	s.NodeConfigs = append(s.NodeConfigs, testnet.NewNodeConfig(flow.RoleVerification,
		testnet.WithID(unittest.IdentifierFixture()),
		testnet.WithLogLevel(zerolog.FatalLevel),
		testnet.AsCorrupted()))

	s.BaseSuite.StartCorruptedNetwork(
		"bft_gossipsub_spam_test",
		10_000,
		100_000,
		func() insecure.AttackOrchestrator {
			s.orchestrator = NewOrchestrator(s.T(), s.Log)
			return s.orchestrator
		},
	)
}
