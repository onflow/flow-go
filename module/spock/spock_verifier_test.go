package spock

import (
	"fmt"
	"testing"

	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/crypto/hash"
	"github.com/onflow/flow-go/model/encoding"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/local"
	realproto "github.com/onflow/flow-go/state/protocol"
	protocol "github.com/onflow/flow-go/state/protocol/mock"
	"github.com/onflow/flow-go/utils/unittest"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

// SpockVerifierTestSuite contains tests against methods of the SpockVerifier scheme
type SpockVerifierTestSuite struct {
	suite.Suite

	// map to hold test idenitities
	identities map[flow.Identifier]*flow.Identity

	snapshot *protocol.Snapshot
	state    *protocol.State

	exeLocal *local.Local
	verLocal *local.Local

	spockHasher hash.Hasher

	verifier *Verifier
}

// TestSpockVerifer invokes all the tests in this test suite
func TestSpockVerifer(t *testing.T) {
	suite.Run(t, new(SpockVerifierTestSuite))
}

// Setup test with n verification nodes
func (vs *SpockVerifierTestSuite) SetupTest() {
	vs.spockHasher = crypto.NewBLSKMAC(encoding.SPOCKTag)

	exe := unittest.IdentityFixture(unittest.WithRole(flow.RoleExecution))
	ver := unittest.IdentityFixture(unittest.WithRole(flow.RoleVerification))

	exeSk, err := unittest.StakingKey()
	require.NoError(vs.T(), err)
	exeLocal, err := local.New(exe, exeSk)
	require.NoError(vs.T(), err)

	verSk, err := unittest.StakingKey()
	require.NoError(vs.T(), err)
	verLocal, err := local.New(ver, verSk)
	require.NoError(vs.T(), err)

	exe.StakingPubKey = exeSk.PublicKey()
	ver.StakingPubKey = verSk.PublicKey()

	vs.exeLocal = exeLocal
	vs.verLocal = verLocal

	// add to identities map
	vs.identities = make(map[flow.Identifier]*flow.Identity)
	vs.identities[vs.exeLocal.NodeID()] = exe
	vs.identities[vs.verLocal.NodeID()] = ver

	vs.snapshot = &protocol.Snapshot{}
	vs.snapshot.On("Identity", mock.Anything).Return(
		func(nodeID flow.Identifier) *flow.Identity {
			identity := vs.identities[nodeID]
			return identity
		},
		func(nodeID flow.Identifier) error {
			_, found := vs.identities[nodeID]
			if !found {
				return fmt.Errorf("could not get identity (%x)", nodeID)
			}
			return nil
		},
	)

	vs.state = &protocol.State{}
	vs.state.On("AtBlockID", mock.Anything).Return(
		func(blockID flow.Identifier) realproto.Snapshot {
			return vs.snapshot
		},
		nil,
	)

	vs.verifier = NewVerifier(vs.state)
}

// TestResultIDNotAdded tests if `verfiier.AddReceipt` creates a map entry if the result.ID() does not exists
func (vs *SpockVerifierTestSuite) TestResultIDNotAdded() {
	receipt := unittest.ExecutionReceiptFixture()

	// add receipt
	err := vs.verifier.AddReceipt(receipt)
	require.NoError(vs.T(), err)

	// check if receipt is added under receipt result id and length is 1
	receipts, ok := vs.verifier.receipts[receipt.ExecutionResult.ID()]
	require.True(vs.T(), ok)
	require.True(vs.T(), len(receipts) == 1)
}

func (vs *SpockVerifierTestSuite) TestAddReceiptWithMatchingSpocks() {
	receipt, _ := vs.ReceiptWithSpocks()

	// add receipt
	err := vs.verifier.AddReceipt(receipt)
	require.NoError(vs.T(), err)

	// check if receipt is added under receipt result id and length is 1
	receipts, ok := vs.verifier.receipts[receipt.ExecutionResult.ID()]
	require.True(vs.T(), ok)
	require.True(vs.T(), len(receipts) == 1)

	// try add again - should be no error as spocks should match
	err = vs.verifier.AddReceipt(receipt)
	require.NoError(vs.T(), err)

	// check if receipts length is still 1
	receipts, ok = vs.verifier.receipts[receipt.ExecutionResult.ID()]
	require.True(vs.T(), ok)
	require.True(vs.T(), len(receipts) == 1)
}

func (vs *SpockVerifierTestSuite) TestAddReceiptWithNonMatchingSpocks() {
	receipt1, _ := vs.ReceiptWithSpocks()
	receipt2, _ := vs.ReceiptWithSpocks()
	receipt3, _ := vs.ReceiptWithSpocks()

	// set the same execution result
	receipt2.ExecutionResult = receipt1.ExecutionResult

	// add receipt
	err := vs.verifier.AddReceipt(receipt1)
	require.NoError(vs.T(), err)

	// check if receipt is added under receipt result id and length is 1
	receipts, ok := vs.verifier.receipts[receipt1.ExecutionResult.ID()]
	require.True(vs.T(), ok)
	require.True(vs.T(), len(receipts) == 1)

	// add another receipt with same result and unmatching spocks
	err = vs.verifier.AddReceipt(receipt2)
	require.NoError(vs.T(), err)

	// check if receipts length is 2
	receipts, ok = vs.verifier.receipts[receipt2.ExecutionResult.ID()]
	require.True(vs.T(), ok)
	require.True(vs.T(), len(receipts) == 2)

	// add another receipt with different result with unmatching spocks
	err = vs.verifier.AddReceipt(receipt3)
	require.NoError(vs.T(), err)

	// check if receipts length is 1
	receipts, ok = vs.verifier.receipts[receipt3.ExecutionResult.ID()]
	require.True(vs.T(), ok)
	require.True(vs.T(), len(receipts) == 1)
}

func (vs *SpockVerifierTestSuite) TestMatchingApprovalSpockWithReceipt() {
	receipt, spockSecret := vs.ReceiptWithSpocks()

	// add receipt to verifier buckets
	err := vs.verifier.AddReceipt(receipt)
	require.NoError(vs.T(), err)

	// check if approval spock matched receipt spocks
	approvals := vs.ApprovalsForReceipt(receipt, spockSecret)
	for _, approval := range approvals {
		verified, err := vs.verifier.VerifyApproval(approval)

		require.NoError(vs.T(), err)
		require.True(vs.T(), verified)
	}
}

// ReceiptWithSpocks generates a receipt with the exe id and generates spocks for each
// of the chunks in the execution result
func (vs *SpockVerifierTestSuite) ReceiptWithSpocks() (*flow.ExecutionReceipt, []byte) {
	// create random spock secret with 10 bytes
	spockSecret := unittest.RandomBytes(10)

	// create receipt with executor
	receipt := unittest.ExecutionReceiptFixture()
	receipt.ExecutorID = vs.exeLocal.NodeID()

	// add spocks for each chunk
	for _, chunk := range receipt.ExecutionResult.Chunks {
		spock, err := vs.exeLocal.SignFunc(spockSecret, vs.spockHasher, crypto.SPOCKProve)
		require.NoError(vs.T(), err)
		receipt.Spocks[chunk.Index] = spock
	}

	return receipt, spockSecret
}

func (vs *SpockVerifierTestSuite) ApprovalsForReceipt(receipt *flow.ExecutionReceipt, spockSecret []byte) []*flow.ResultApproval {
	approvals := make([]*flow.ResultApproval, 0, receipt.ExecutionResult.Chunks.Len())

	for _, chunk := range receipt.ExecutionResult.Chunks {
		approval := unittest.ResultApprovalFixture()
		approval.Body.ChunkIndex = chunk.Index
		approval.Body.ExecutionResultID = receipt.ExecutionResult.ID()
		approval.Body.ApproverID = vs.verLocal.NodeID()

		spock, err := vs.verLocal.SignFunc(spockSecret, vs.spockHasher, crypto.SPOCKProve)
		require.NoError(vs.T(), err)
		approval.Body.Spock = spock

		approvals = append(approvals, approval)
	}

	return approvals
}
