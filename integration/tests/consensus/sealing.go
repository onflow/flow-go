package consensus

import (
	"context"
	"math/rand"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/engine"
	exeUtils "github.com/onflow/flow-go/engine/execution/utils"
	"github.com/onflow/flow-go/engine/ghost/client"
	verUtils "github.com/onflow/flow-go/engine/verification/utils"
	"github.com/onflow/flow-go/integration/testnet"
	"github.com/onflow/flow-go/integration/tests/common"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/messages"
	"github.com/onflow/flow-go/utils/unittest"
)

type SealingSuite struct {
	suite.Suite
	log    zerolog.Logger
	cancel context.CancelFunc
	net    *testnet.FlowNetwork
	conIDs []flow.Identifier
	exeID  flow.Identifier
	exe2ID flow.Identifier
	exeSK  crypto.PrivateKey
	exe2SK crypto.PrivateKey
	verID  flow.Identifier
	verSK  crypto.PrivateKey
	reader *client.FlowMessageStreamReader
}

func (ss *SealingSuite) Execution() *client.GhostClient {
	ghost := ss.net.ContainerByID(ss.exeID)
	client, err := common.GetGhostClient(ghost)
	require.NoError(ss.T(), err, "could not get ghost client")
	return client
}

func (ss *SealingSuite) Execution2() *client.GhostClient {
	ghost := ss.net.ContainerByID(ss.exe2ID)
	client, err := common.GetGhostClient(ghost)
	require.NoError(ss.T(), err, "could not get ghost client")
	return client
}

func (ss *SealingSuite) Verification() *client.GhostClient {
	ghost := ss.net.ContainerByID(ss.verID)
	client, err := common.GetGhostClient(ghost)
	require.NoError(ss.T(), err, "could not get ghost client")
	return client
}

func (ss *SealingSuite) SetupTest() {
	logger := unittest.LoggerWithLevel(zerolog.InfoLevel).With().
		Str("testfile", "sealing.go").
		Str("testcase", ss.T().Name()).
		Logger()
	ss.log = logger
	ss.log.Info().Msgf("================> SetupTest")

	// seed random generator
	rand.Seed(time.Now().UnixNano())

	// to collect node confiss...
	var nodeConfigs []testnet.NodeConfig

	// need one dummy collection node (unused ghost)
	colConfig := testnet.NewNodeConfig(flow.RoleCollection, testnet.WithLogLevel(zerolog.FatalLevel), testnet.AsGhost())
	nodeConfigs = append(nodeConfigs, colConfig)

	// need three real consensus nodes
	for n := 0; n < 3; n++ {
		conID := unittest.IdentifierFixture()
		nodeConfig := testnet.NewNodeConfig(flow.RoleConsensus, testnet.WithLogLevel(zerolog.InfoLevel), testnet.WithID(conID))
		nodeConfigs = append(nodeConfigs, nodeConfig)
		ss.conIDs = append(ss.conIDs, conID)
	}
	ss.log.Info().Msgf("consensus IDs: %v\n", ss.conIDs)

	// need one controllable execution node (used ghost)
	ss.exeID = unittest.IdentifierFixture()
	exeConfig := testnet.NewNodeConfig(flow.RoleExecution, testnet.WithLogLevel(zerolog.FatalLevel), testnet.WithID(ss.exeID), testnet.AsGhost())
	nodeConfigs = append(nodeConfigs, exeConfig)

	// need another controllable execution node (used ghost) in order to
	// send matching execution receipts
	ss.exe2ID = unittest.IdentifierFixture()
	exe2Config := testnet.NewNodeConfig(flow.RoleExecution, testnet.WithLogLevel(zerolog.FatalLevel), testnet.WithID(ss.exe2ID), testnet.AsGhost())
	nodeConfigs = append(nodeConfigs, exe2Config)

	// need one controllable verification node (used ghost)
	ss.verID = unittest.IdentifierFixture()
	verConfig := testnet.NewNodeConfig(flow.RoleVerification, testnet.WithLogLevel(zerolog.FatalLevel), testnet.WithID(ss.verID), testnet.AsGhost())
	nodeConfigs = append(nodeConfigs, verConfig)
	ss.log.Info().Msgf("verification ID: %v\n", ss.verID)

	nodeConfigs = append(nodeConfigs,
		testnet.NewNodeConfig(flow.RoleAccess, testnet.WithLogLevel(zerolog.FatalLevel)),
	)

	// generate the network config
	netConfig := testnet.NewNetworkConfig("consensus_execution_state_sealing", nodeConfigs)

	// initialize the network
	ss.net = testnet.PrepareFlowNetwork(ss.T(), netConfig)

	keys, err := ss.net.ContainerByID(ss.exeID).Config.NodeInfo.PrivateKeys()
	require.NoError(ss.T(), err)
	ss.exeSK = keys.StakingKey

	keys, err = ss.net.ContainerByID(ss.exe2ID).Config.NodeInfo.PrivateKeys()
	require.NoError(ss.T(), err)
	ss.exe2SK = keys.StakingKey

	keys, err = ss.net.ContainerByID(ss.verID).Config.NodeInfo.PrivateKeys()
	require.NoError(ss.T(), err)
	ss.verSK = keys.StakingKey

	// start the network
	ctx, cancel := context.WithCancel(context.Background())
	ss.cancel = cancel
	ss.net.Start(ctx)

	// subscribe to the ghost
	for attempts := 0; ; attempts++ {
		var err error
		ss.reader, err = ss.Execution().Subscribe(context.Background()) // doesn't matter which one
		if err == nil {
			break
		}
		if attempts >= 10 {
			require.NoError(ss.T(), err, "could not subscribe to ghost (%d attempts)", attempts)
		}
	}
}

func (ss *SealingSuite) TearDownTest() {
	ss.log.Info().Msgf("================> Start TearDownTest")
	ss.net.Remove()
	ss.cancel()
	ss.log.Info().Msgf("================> Finish TearDownTest")
}

func (ss *SealingSuite) TestBlockSealCreation() {
	ss.log.Info().Msgf("================> RUNNING TESTING")

	// fix the deadline of the entire test
	deadline := time.Now().Add(30 * time.Second)
	ss.log.Info().Msgf("seal creation deadline %s", deadline)

	// first, we listen to see which block proposal is the first one to be
	// confirmed three times (finalized)
	var targetID flow.Identifier
	parents := make(map[flow.Identifier]flow.Identifier)
	confirmations := make(map[flow.Identifier]uint)
SearchLoop:
	for time.Now().Before(deadline) {

		// we read the next message until we reach deadline
		_, msg, err := ss.reader.Next()
		if err != nil {
			ss.T().Logf("could not read next message: %s\n", err)
			continue
		}

		// we only care about block proposals at the moment
		proposal, ok := msg.(*messages.BlockProposal)
		if !ok {
			continue
		}

		// make sure we skip duplicates
		proposalID := proposal.Header.ID()
		_, processed := confirmations[proposalID]
		if processed {
			continue
		}
		confirmations[proposalID] = 0

		// we map the proposal to its parent for later
		parentID := proposal.Header.ParentID
		parents[proposalID] = parentID

		ss.T().Logf("received block proposal height %v, view %v, id %v",
			proposal.Header.Height,
			proposal.Header.View,
			proposalID)

		// we add one confirmation for each ancestor
		for {

			// check if we keep track of the parent
			_, tracked := confirmations[parentID]
			if !tracked {
				break
			}

			// add one confirmation to parent
			confirmations[parentID]++

			// check if we reached three confirmations
			if confirmations[parentID] >= 3 {
				targetID = parentID
				break SearchLoop
			}

			// check if there is another ancestor
			parentID, tracked = parents[parentID]
			if !tracked {
				break
			}
		}
	}

	// make sure we found a target block to seal
	require.NotEqual(ss.T(), flow.ZeroID, targetID, "should have found target block")

	ss.T().Logf("target for sealing found (block: %x)\n", targetID)

	seal := ss.net.Seal()
	resultID := seal.ResultID

	// create a chunk for the execution result
	chunk := flow.Chunk{
		ChunkBody: flow.ChunkBody{
			CollectionIndex:      0, // irrelevant for consensus node
			StartState:           seal.FinalState,
			EventCollection:      flow.ZeroID, // irrelevant for consensus node
			BlockID:              targetID,
			TotalComputationUsed: 0, // irrelevant for consensus node
			NumberOfTransactions: 0, // irrelevant for consensus node
		},
		Index:    0,                                 // should start at zero
		EndState: unittest.StateCommitmentFixture(), // random end execution state
	}

	// create the execution result for the target block
	result := flow.ExecutionResult{
		PreviousResultID: resultID,               // need genesis result
		BlockID:          targetID,               // refer the target block
		Chunks:           flow.ChunkList{&chunk}, // include only chunk
	}

	ss.T().Logf("execution result generated (result: %x)\n", result.ID())

	// create the execution receipt for the only execution node
	receipt := flow.ExecutionReceipt{
		ExecutorID:        ss.exeID, // our fake execution node
		ExecutionResult:   result,   // result for target block
		Spocks:            nil,      // ignored
		ExecutorSignature: crypto.Signature{},
	}

	// generates a signature over the execution result
	id := receipt.ID()
	sig, err := ss.exeSK.Sign(id[:], exeUtils.NewExecutionReceiptHasher())
	require.NoError(ss.T(), err)

	receipt.ExecutorSignature = sig

	// keep trying to send 2 matching execution receipt to the first consensus node
	receipt2 := flow.ExecutionReceipt{
		ExecutorID:        ss.exe2ID, // our fake execution node
		ExecutionResult:   result,    // result for target block
		Spocks:            nil,       // ignored
		ExecutorSignature: crypto.Signature{},
	}

	id = receipt2.ID()
	sig2, err := ss.exe2SK.Sign(id[:], exeUtils.NewExecutionReceiptHasher())
	require.NoError(ss.T(), err)

	receipt2.ExecutorSignature = sig2

	valid, err := ss.exe2SK.PublicKey().Verify(receipt2.ExecutorSignature, id[:], exeUtils.NewExecutionReceiptHasher())
	require.NoError(ss.T(), err)
	require.True(ss.T(), valid)

ReceiptLoop:
	for time.Now().Before(deadline) {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		err := ss.Execution().Send(ctx, engine.PushReceipts, &receipt, ss.conIDs...)
		err = ss.Execution2().Send(ctx, engine.PushReceipts, &receipt2, ss.conIDs...)
		cancel()
		if err != nil {
			ss.T().Logf("could not send execution receipt: %s\n", err)
			continue
		}
		break ReceiptLoop
	}

	ss.T().Logf("execution receipt submitted (receipt: %x, result: %x)\n", receipt.ID(), receipt.ExecutionResult.ID())
	ss.T().Logf("execution receipt submitted (receipt2: %x, result: %x)\n", receipt2.ID(), receipt2.ExecutionResult.ID())

	// attestation
	atst := flow.Attestation{
		BlockID:           result.BlockID, // the block for the result
		ExecutionResultID: result.ID(),    // the actual execution result
		ChunkIndex:        chunk.Index,    // the chunk of this approval
	}

	// generates a signature over the attestation part of approval
	atstID := atst.ID()
	atstSign, err := ss.verSK.Sign(atstID[:], verUtils.NewResultApprovalHasher())
	require.NoError(ss.T(), err)

	// result approval body
	body := flow.ResultApprovalBody{
		Attestation:          atst,
		ApproverID:           ss.verID,
		AttestationSignature: atstSign,
		Spock:                nil,
	}

	// generates a signature over result approval body
	bodyID := body.ID()
	bodySign, err := ss.verSK.Sign(bodyID[:], verUtils.NewResultApprovalHasher())
	require.NoError(ss.T(), err)

	approval := flow.ResultApproval{
		Body:              body,
		VerifierSignature: bodySign,
	}

	// keep trying to send result approval to the first consensus node
	ss.T().Log("sending result approval")
ApprovalLoop:
	for time.Now().Before(deadline) {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		err := ss.Verification().Send(ctx, engine.PushApprovals, &approval, ss.conIDs...)
		cancel()
		if err != nil {
			ss.T().Logf("could not send result approval: %s\n", err)
			continue
		}
		break ApprovalLoop
	}

	ss.T().Logf("result approval submitted (approval: %x, result: %x)\n", approval.ID(), approval.Body.ExecutionResultID)

	// we try to find a block with the guarantee included and three confirmations
	found := false
SealingLoop:
	for time.Now().Before(deadline) {

		// we read the next message until we reach deadline
		_, msg, err := ss.reader.Next()
		if err != nil {
			ss.T().Logf("could not read message: %s\n", err)
			continue
		}

		// we only care about block proposals at the moment
		proposal, ok := msg.(*messages.BlockProposal)
		if !ok {
			continue
		}

		// log the proposal details
		proposalID := proposal.Header.ID()
		seals := proposal.Payload.Seals

		// if the block seal is included, we add the block to those we
		// monitor for confirmations
		for _, seal := range seals {
			if seal.ResultID == result.ID() {
				found = true
				ss.T().Logf("%x: block seal included!\n", proposalID)
				break SealingLoop
			}
		}
	}

	// make sure we found the guarantee in at least one block proposal
	require.True(ss.T(), found, "block seal should have been included in at least one block")
}
