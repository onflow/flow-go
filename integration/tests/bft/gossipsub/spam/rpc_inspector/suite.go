package rpc_inspector

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
	attackerVNIDNoSigning   flow.Identifier // corrupt attacker EN id, this node has message signing disabled
	attackerVNIDWithSigning flow.Identifier // corrupt attacker EN id, this node has message signing enabled
	victimENID              flow.Identifier // corrupt attacker VN id
	Orchestrator            *orchestrator
}

func (s *Suite) SetupSuite() {
	s.BaseSuite.SetupSuite()

	// create 1 victim VN node
	s.attackerVNIDNoSigning = unittest.IdentifierFixture()
	s.NodeConfigs = append(s.NodeConfigs, testnet.NewNodeConfig(flow.RoleVerification,
		testnet.WithID(s.attackerVNIDNoSigning),
		testnet.WithLogLevel(zerolog.FatalLevel),
		testnet.AsCorrupted()))

	s.BaseSuite.StartCorruptedNetwork(
		"bft_rpc_inspector_test",
		10_000,
		100_000,
		func() insecure.AttackOrchestrator {
			s.Orchestrator = NewRPCOrchestrator(s.T(), s.Log)
			return s.Orchestrator
		},
	)
}
