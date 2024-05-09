package disallowlisting

import (
	"context"
	"fmt"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/insecure"
	"github.com/onflow/flow-go/integration/client"
	"github.com/onflow/flow-go/integration/testnet"
	"github.com/onflow/flow-go/integration/tests/bft"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

// Suite represents a test suite ensures the admin block list command works as expected.
type Suite struct {
	bft.BaseSuite
	senderVN     flow.Identifier // node ID of corrupted node that will send messages in the test. The sender node will be blocked.
	receiverEN   flow.Identifier // node ID of corrupted node that will receive messages in the test
	Orchestrator *Orchestrator
}

// SetupSuite runs a bare minimum Flow network to function correctly along with 2 attacker nodes and 1 victim node.
// - Corrupt VN that will be used to send messages, this node will be the node that is blocked by the receiver corrupt EN.
// - Corrupt EN that will receive messages from the corrupt VN, we will execute the admin command on this node.
func (s *Suite) SetupSuite() {
	s.BaseSuite.SetupSuite()

	// generate 1 corrupt verification node
	s.senderVN = unittest.IdentifierFixture()
	s.NodeConfigs = append(s.NodeConfigs, testnet.NewNodeConfig(flow.RoleVerification,
		testnet.WithID(s.senderVN),
		testnet.WithLogLevel(zerolog.FatalLevel),
		testnet.AsCorrupted()))

	// generate 1 corrupt execution node
	s.receiverEN = unittest.IdentifierFixture()
	s.NodeConfigs = append(s.NodeConfigs, testnet.NewNodeConfig(flow.RoleExecution,
		testnet.WithID(s.receiverEN),
		testnet.WithLogLevel(zerolog.FatalLevel),
		testnet.AsCorrupted()))

	s.BaseSuite.StartCorruptedNetwork(
		"bft_signature_validation_test",
		10_000,
		100_000,
		func() insecure.AttackOrchestrator {
			s.Orchestrator = NewOrchestrator(s.T(), s.Log, s.senderVN, s.receiverEN)
			return s.Orchestrator
		},
	)
}

// disallowListNode submit request to our EN admin server to block sender VN.
func (s *Suite) disallowListNode(nodeID flow.Identifier) {
	serverAddr := fmt.Sprintf("localhost:%s", s.Net.ContainerByID(s.receiverEN).Port(testnet.AdminPort))
	adminClient := client.NewAdminClient(serverAddr)

	data := map[string]interface{}{"network-id-provider-blocklist": []string{nodeID.String()}}
	resp, err := adminClient.RunCommand(context.Background(), "set-config", data)
	require.NoError(s.T(), err)

	output, ok := resp.Output.(map[string]interface{})
	require.True(s.T(), ok)

	newList, ok := output["newValue"].([]interface{})
	require.True(s.T(), ok)
	require.Contains(s.T(), newList, nodeID.String())
}
