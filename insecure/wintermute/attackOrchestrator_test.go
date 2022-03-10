package wintermute

import (
	"context"
	mockinsecure "github.com/onflow/flow-go/insecure/mock"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/utils/unittest"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestWintermuteOrchestrator(t *testing.T) {
	// create corrupted Wintermute VN, EN nodes
	colludingVNIdentityList := unittest.IdentityListFixture(3, unittest.WithRole(flow.RoleVerification))
	maliciousENIdentityList := unittest.IdentityListFixture(2, unittest.WithRole(flow.RoleExecution))
	corruptedIdentityList := append(colludingVNIdentityList, maliciousENIdentityList...)

	// create all honest nodes
	honestIdentityList := unittest.IdentityListFixture(5, unittest.WithAllRoles())

	// create list of all nodes - honest and malicious
	allIdentityList := append(honestIdentityList, corruptedIdentityList...)

	// should be mocked network
	attackNetwork := &mockinsecure.AttackNetwork{}

	orchestrator := NewOrchestrator(allIdentityList, corruptedIdentityList, attackNetwork, unittest.Logger())

	attackNetwork.On("Start", mock.AnythingOfType("*irrecoverable.signalerCtx")).Return().Once()
	signalerCtx, _ := irrecoverable.WithSignaler(context.Background())
	orchestrator.start(signalerCtx)

	//genesisFixture := unittest.GenesisFixture()

	//honestExecutionResult1 := unittest.ExecutionResultFixture(unittest.WithBlock(genesisFixture))
	//honestExecutionResult2 := unittest.ExecutionResultFixture(unittest.WithBlock(genesisFixture))
	//
	//require.Equal(t, honestExecutionResult1.ID(), honestExecutionResult2.ID())

	blockIDFixture := unittest.IdentifierFixture()
	honestExecutionResult1 := unittest.ExecutionResultFixture(unittest.WithExecutionResultBlockID(blockIDFixture))
	honestExecutionResult2 := unittest.ExecutionResultFixture(unittest.WithExecutionResultBlockID(blockIDFixture))
	require.Equal(t, honestExecutionResult1.ID(), honestExecutionResult2.ID())

	//honestExecutionResult1.ID() // use this to check if execution results are the same
	//honestExecutionReceipt1 := unittest.ExecutionReceiptFixture(unittest.WithResult(honestExecutionResult1))
	//honestExecutionReceipt2 := unittest.ExecutionReceiptFixture(unittest.WithResult(honestExecutionResult1))

	// corrupt execution result
	honestExecutionResult1.Chunks[0].CollectionIndex = 999

	//require.Equal(t, honestExecutionReceipt1, honestExecutionReceipt2)

	//if honestExecutionResult1 1 and honestExecutionResult1 2 are agreeging
	//orchestrator.HandleEventFromCorruptedNode(executionResultFixtureFromNode1)
	//orchestrator.HandleEventFromCorruptedNode(executionResultFixtureFromNode2)

	// orchestrator replaces honestExecutionResult1 1 and honestExecutionResult1 2 with corrupted agreeing ones and send them over network
	// to execution node 1 and execution 2, respectively.
	//attackNetwork.On("")
}
