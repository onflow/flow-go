package unittest

import (
	"fmt"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/model/chunks"
	"github.com/onflow/flow-go/model/flow"
	mempool "github.com/onflow/flow-go/module/mempool/mock"
	module "github.com/onflow/flow-go/module/mock"
	realproto "github.com/onflow/flow-go/state/protocol"
	protocol "github.com/onflow/flow-go/state/protocol/mock"
	storerr "github.com/onflow/flow-go/storage"
	storage "github.com/onflow/flow-go/storage/mock"
)

type BaseChainSuite struct {
	suite.Suite

	// IDENTITIES
	ConID flow.Identifier
	ExeID flow.Identifier
	VerID flow.Identifier

	Identities map[flow.Identifier]*flow.Identity
	Approvers  flow.IdentityList

	// BLOCKS
	RootBlock            flow.Block
	LatestSealedBlock    flow.Block
	LatestFinalizedBlock *flow.Block
	UnfinalizedBlock     flow.Block
	Blocks               map[flow.Identifier]*flow.Block

	// PROTOCOL STATE
	State          *protocol.State
	SealedSnapshot *protocol.Snapshot
	FinalSnapshot  *protocol.Snapshot

	// MEMPOOLS and STORAGE which are injected into Matching Engine
	// mock storage.ExecutionReceipts: backed by in-memory map PersistedReceipts
	ReceiptsDB             *storage.ExecutionReceipts
	PersistedReceipts      map[flow.Identifier]*flow.ExecutionReceipt
	PersistedReceiptsIndex map[flow.Identifier]flow.Identifier // index ExecutionResult.BlockID -> ExecutionReceipt.ID

	ResultsDB        *storage.ExecutionResults
	PersistedResults map[flow.Identifier]*flow.ExecutionResult

	// mock mempool.IncorporatedResults: backed by in-memory map PendingResults
	ResultsPL      *mempool.IncorporatedResults
	PendingResults map[flow.Identifier]*flow.IncorporatedResult

	// mock mempool.IncorporatedResultSeals: backed by in-memory map PendingSeals
	SealsPL      *mempool.IncorporatedResultSeals
	PendingSeals map[flow.Identifier]*flow.IncorporatedResultSeal

	// mock BLOCK STORAGE: backed by in-memory map Blocks
	HeadersDB  *storage.Headers               // backed by map Blocks
	IndexDB    *storage.Index                 // backed by map Blocks
	PayloadsDB *storage.Payloads              // backed by map Blocks
	SealsDB    *storage.Seals                 // backed by map SealsIndex
	SealsIndex map[flow.Identifier]*flow.Seal // last valid seal for block

	// mock mempool.Approvals: used to test whether or not Matching Engine stores approvals
	// mock storage backed by in-memory map PendingApprovals
	ApprovalsPL      *mempool.Approvals
	PendingApprovals map[flow.Identifier]map[uint64]map[flow.Identifier]*flow.ResultApproval

	// mock mempool.ReceiptsForest: used to test whether or not Matching Engine stores receipts
	ReceiptsPL *mempool.ExecutionTree

	Assigner    *module.ChunkAssigner
	Assignments map[flow.Identifier]*chunks.Assignment // index for assignments for given execution result
}

func (bc *BaseChainSuite) SetupChain() {

	// ~~~~~~~~~~~~~~~~~~~~~~~~~~ SETUP IDENTITIES ~~~~~~~~~~~~~~~~~~~~~~~~~~ //

	// asign node Identities
	con := IdentityFixture(WithRole(flow.RoleConsensus))
	exe := IdentityFixture(WithRole(flow.RoleExecution))
	ver := IdentityFixture(WithRole(flow.RoleVerification))

	bc.ConID = con.NodeID
	bc.ExeID = exe.NodeID
	bc.VerID = ver.NodeID

	bc.Identities = make(map[flow.Identifier]*flow.Identity)
	bc.Identities[bc.ConID] = con
	bc.Identities[bc.ExeID] = exe
	bc.Identities[bc.VerID] = ver

	// assign 4 nodes to the verification role
	bc.Approvers = IdentityListFixture(4, WithRole(flow.RoleVerification))
	for _, verifier := range bc.Approvers {
		bc.Identities[verifier.ID()] = verifier
	}

	// ~~~~~~~~~~~~~~~~~~~~~~~~~~~~ SETUP BLOCKS ~~~~~~~~~~~~~~~~~~~~~~~~~~~~ //
	// RootBlock <- LatestSealedBlock <- LatestFinalizedBlock <- UnfinalizedBlock
	bc.RootBlock = BlockFixture()
	bc.LatestSealedBlock = BlockWithParentFixture(bc.RootBlock.Header)
	latestFinalizedBlock := BlockWithParentFixture(bc.LatestSealedBlock.Header)
	bc.LatestFinalizedBlock = &latestFinalizedBlock
	bc.UnfinalizedBlock = BlockWithParentFixture(bc.LatestFinalizedBlock.Header)

	bc.Blocks = make(map[flow.Identifier]*flow.Block)
	bc.Blocks[bc.RootBlock.ID()] = &bc.RootBlock
	bc.Blocks[bc.LatestSealedBlock.ID()] = &bc.LatestSealedBlock
	bc.Blocks[bc.LatestFinalizedBlock.ID()] = bc.LatestFinalizedBlock
	bc.Blocks[bc.UnfinalizedBlock.ID()] = &bc.UnfinalizedBlock

	// ~~~~~~~~~~~~~~~~~~~~~~~~ SETUP PROTOCOL STATE ~~~~~~~~~~~~~~~~~~~~~~~~ //
	bc.State = &protocol.State{}

	// define the protocol state snapshot of the latest finalized block
	bc.State.On("Final").Return(
		func() realproto.Snapshot {
			return bc.FinalSnapshot
		},
		nil,
	)
	bc.FinalSnapshot = &protocol.Snapshot{}
	bc.FinalSnapshot.On("Head").Return(
		func() *flow.Header {
			return bc.LatestFinalizedBlock.Header
		},
		nil,
	)

	// define the protocol state snapshot of the latest sealed block
	bc.State.On("Sealed").Return(
		func() realproto.Snapshot {
			return bc.SealedSnapshot
		},
		nil,
	)
	bc.SealedSnapshot = &protocol.Snapshot{}
	bc.SealedSnapshot.On("Head").Return(
		func() *flow.Header {
			return bc.LatestSealedBlock.Header
		},
		nil,
	)

	findBlockByHeight := func(blocks map[flow.Identifier]*flow.Block, height uint64) (*flow.Block, bool) {
		for _, block := range blocks {
			if block.Header.Height == height {
				return block, true
			}
		}
		return nil, false
	}

	// define the protocol state snapshot for any block in `bc.Blocks`
	bc.State.On("AtBlockID", mock.Anything).Return(
		func(blockID flow.Identifier) realproto.Snapshot {
			block, found := bc.Blocks[blockID]
			if !found {
				return StateSnapshotForUnknownBlock()
			}
			return StateSnapshotForKnownBlock(block.Header, bc.Identities)
		},
	)

	bc.State.On("AtHeight", mock.Anything).Return(
		func(height uint64) realproto.Snapshot {
			block, found := findBlockByHeight(bc.Blocks, height)
			if found {
				snapshot := &protocol.Snapshot{}
				snapshot.On("Head").Return(
					func() *flow.Header {
						return block.Header
					},
					nil,
				)
				return snapshot
			}
			panic(fmt.Sprintf("unknown height: %v, final: %v, sealed: %v", height, bc.LatestFinalizedBlock.Header.Height, bc.LatestSealedBlock.Header.Height))
		},
	)

	// ~~~~~~~~~~~~~~~~~~~~~~~ SETUP RESULTS STORAGE ~~~~~~~~~~~~~~~~~~~~~~~~ //
	bc.PersistedResults = make(map[flow.Identifier]*flow.ExecutionResult)
	bc.ResultsDB = &storage.ExecutionResults{}
	bc.ResultsDB.On("ByID", mock.Anything).Return(
		func(resultID flow.Identifier) *flow.ExecutionResult {
			return bc.PersistedResults[resultID]
		},
		func(resultID flow.Identifier) error {
			_, found := bc.PersistedResults[resultID]
			if !found {
				return storerr.ErrNotFound
			}
			return nil
		},
	).Maybe()
	bc.ResultsDB.On("Store", mock.Anything).Return(
		func(result *flow.ExecutionResult) error {
			_, found := bc.PersistedResults[result.ID()]
			if found {
				return storerr.ErrAlreadyExists
			}
			return nil
		},
	).Maybe() // this call is optional
	// ~~~~~~~~~~~~~~~~~~~~~~~ SETUP RECEIPTS STORAGE ~~~~~~~~~~~~~~~~~~~~~~~~ //
	bc.PersistedReceipts = make(map[flow.Identifier]*flow.ExecutionReceipt)
	bc.PersistedReceiptsIndex = make(map[flow.Identifier]flow.Identifier)
	bc.ReceiptsDB = &storage.ExecutionReceipts{}
	bc.ReceiptsDB.On("ByID", mock.Anything).Return(
		func(receiptID flow.Identifier) *flow.ExecutionReceipt {
			return bc.PersistedReceipts[receiptID]
		},
		func(receiptID flow.Identifier) error {
			_, found := bc.PersistedReceipts[receiptID]
			if !found {
				return storerr.ErrNotFound
			}
			return nil
		},
	).Maybe()
	bc.ReceiptsDB.On("Index", mock.Anything, mock.Anything).Return(
		func(blockID flow.Identifier, receiptID flow.Identifier) error {
			_, found := bc.PersistedReceiptsIndex[receiptID]
			if found {
				return storerr.ErrAlreadyExists
			}
			bc.PersistedReceiptsIndex[blockID] = receiptID
			return nil
		},
	)
	bc.ReceiptsDB.On("ByBlockID", mock.Anything).Return(
		func(blockID flow.Identifier) *flow.ExecutionReceipt {
			receiptID, found := bc.PersistedReceiptsIndex[blockID]
			if !found {
				return nil
			}
			return bc.PersistedReceipts[receiptID]
		},
		func(blockID flow.Identifier) error {
			_, found := bc.PersistedReceiptsIndex[blockID]
			if !found {
				return storerr.ErrNotFound
			}
			return nil
		},
	).Maybe()
	bc.ReceiptsDB.On("Store", mock.Anything).Return(
		func(receipt *flow.ExecutionReceipt) error {
			_, found := bc.PersistedReceipts[receipt.ID()]
			if found {
				return storerr.ErrAlreadyExists
			}
			return nil
		},
	).Maybe() // this call is optional
	bc.ReceiptsDB.On("ByBlockIDAllExecutionReceipts", mock.Anything).Return(
		func(blockID flow.Identifier) []*flow.ExecutionReceipt {
			var receipts []*flow.ExecutionReceipt
			return receipts
		},
		func(blockID flow.Identifier) error {
			return nil
		},
	).Maybe()

	// ~~~~~~~~~~~~~~~~~~~~ SETUP BLOCK HEADER STORAGE ~~~~~~~~~~~~~~~~~~~~~ //
	bc.HeadersDB = &storage.Headers{}
	bc.HeadersDB.On("ByBlockID", mock.Anything).Return(
		func(blockID flow.Identifier) *flow.Header {
			block, found := bc.Blocks[blockID]
			if !found {
				return nil
			}
			return block.Header
		},
		func(blockID flow.Identifier) error {
			_, found := bc.Blocks[blockID]
			if !found {
				return storerr.ErrNotFound
			}
			return nil
		},
	)
	bc.HeadersDB.On("ByHeight", mock.Anything).Return(
		func(blockHeight uint64) *flow.Header {
			for _, b := range bc.Blocks {
				if b.Header.Height == blockHeight {
					return b.Header
				}
			}
			return nil
		},
		func(blockHeight uint64) error {
			for _, b := range bc.Blocks {
				if b.Header.Height == blockHeight {
					return nil
				}
			}
			return storerr.ErrNotFound
		},
	)

	// ~~~~~~~~~~~~~~~~~~~~ SETUP BLOCK PAYLOAD STORAGE ~~~~~~~~~~~~~~~~~~~~~ //
	bc.IndexDB = &storage.Index{}
	bc.IndexDB.On("ByBlockID", mock.Anything).Return(
		func(blockID flow.Identifier) *flow.Index {
			block, found := bc.Blocks[blockID]
			if !found {
				return nil
			}
			if block.Payload == nil {
				return nil
			}
			return block.Payload.Index()
		},
		func(blockID flow.Identifier) error {
			block, found := bc.Blocks[blockID]
			if !found {
				return storerr.ErrNotFound
			}
			if block.Payload == nil {
				return storerr.ErrNotFound
			}
			return nil
		},
	)

	bc.SealsIndex = make(map[flow.Identifier]*flow.Seal)
	firtSeal := Seal.Fixture(Seal.WithBlock(bc.LatestSealedBlock.Header))
	for id, block := range bc.Blocks {
		if id != bc.RootBlock.ID() {
			bc.SealsIndex[block.ID()] = firtSeal
		}
	}

	bc.PayloadsDB = &storage.Payloads{}
	bc.PayloadsDB.On("ByBlockID", mock.Anything).Return(
		func(blockID flow.Identifier) *flow.Payload {
			block, found := bc.Blocks[blockID]
			if !found {
				return nil
			}
			if block.Payload == nil {
				return nil
			}
			return block.Payload
		},
		func(blockID flow.Identifier) error {
			block, found := bc.Blocks[blockID]
			if !found {
				return storerr.ErrNotFound
			}
			if block.Payload == nil {
				return storerr.ErrNotFound
			}
			return nil
		},
	)

	bc.SealsDB = &storage.Seals{}
	bc.SealsDB.On("ByBlockID", mock.Anything).Return(
		func(blockID flow.Identifier) *flow.Seal {
			seal, found := bc.SealsIndex[blockID]
			if !found {
				return nil
			}
			return seal
		},
		func(blockID flow.Identifier) error {
			seal, found := bc.SealsIndex[blockID]
			if !found {
				return storerr.ErrNotFound
			}
			if seal == nil {
				return storerr.ErrNotFound
			}
			return nil
		},
	)

	// ~~~~~~~~~~~~~~~~ SETUP INCORPORATED RESULTS MEMPOOL ~~~~~~~~~~~~~~~~~ //
	bc.PendingResults = make(map[flow.Identifier]*flow.IncorporatedResult)
	bc.ResultsPL = &mempool.IncorporatedResults{}
	bc.ResultsPL.On("Size").Return(uint(0)).Maybe() // only for metrics
	bc.ResultsPL.On("All").Return(
		func() []*flow.IncorporatedResult {
			results := make([]*flow.IncorporatedResult, 0, len(bc.PendingResults))
			for _, result := range bc.PendingResults {
				results = append(results, result)
			}
			return results
		},
	).Maybe()

	// ~~~~~~~~~~~~~~~~~~~~~~ SETUP APPROVALS MEMPOOL ~~~~~~~~~~~~~~~~~~~~~~ //
	bc.ApprovalsPL = &mempool.Approvals{}
	bc.ApprovalsPL.On("Size").Return(uint(0)).Maybe() // only for metrics
	bc.PendingApprovals = make(map[flow.Identifier]map[uint64]map[flow.Identifier]*flow.ResultApproval)
	bc.ApprovalsPL.On("ByChunk", mock.Anything, mock.Anything).Return(
		func(resultID flow.Identifier, chunkIndex uint64) map[flow.Identifier]*flow.ResultApproval {
			return bc.PendingApprovals[resultID][chunkIndex]
		},
	).Maybe()

	// ~~~~~~~~~~~~~~~~~~~~~~~ SETUP RECEIPTS MEMPOOL ~~~~~~~~~~~~~~~~~~~~~~ //
	bc.ReceiptsPL = &mempool.ExecutionTree{}
	bc.ReceiptsPL.On("Size").Return(uint(0)).Maybe() // only for metrics

	// ~~~~~~~~~~~~~~~~~~~~~~~~ SETUP SEALS MEMPOOL ~~~~~~~~~~~~~~~~~~~~~~~~ //
	bc.PendingSeals = make(map[flow.Identifier]*flow.IncorporatedResultSeal)
	bc.SealsPL = &mempool.IncorporatedResultSeals{}
	bc.SealsPL.On("Size").Return(uint(0)).Maybe() // only for metrics
	bc.SealsPL.On("Limit").Return(uint(1000)).Maybe()
	bc.SealsPL.On("ByID", mock.Anything).Return(
		func(sealID flow.Identifier) *flow.IncorporatedResultSeal {
			return bc.PendingSeals[sealID]
		},
		func(sealID flow.Identifier) bool {
			_, found := bc.PendingSeals[sealID]
			return found
		},
	).Maybe()

	bc.Assigner = &module.ChunkAssigner{}
	bc.Assignments = make(map[flow.Identifier]*chunks.Assignment)
}

func StateSnapshotForUnknownBlock() *protocol.Snapshot {
	snapshot := &protocol.Snapshot{}
	snapshot.On("Identity", mock.Anything).Return(
		nil, storerr.ErrNotFound,
	)
	snapshot.On("Head", mock.Anything).Return(
		nil, storerr.ErrNotFound,
	)
	return snapshot
}

func StateSnapshotForKnownBlock(block *flow.Header, identities map[flow.Identifier]*flow.Identity) *protocol.Snapshot {
	snapshot := &protocol.Snapshot{}
	snapshot.On("Identity", mock.Anything).Return(
		func(nodeID flow.Identifier) *flow.Identity {
			return identities[nodeID]
		},
		func(nodeID flow.Identifier) error {
			_, found := identities[nodeID]
			if !found {
				return realproto.IdentityNotFoundError{NodeID: nodeID}
			}
			return nil
		},
	)
	snapshot.On("Head").Return(block, nil)
	return snapshot
}

func ApprovalFor(result *flow.ExecutionResult, chunkIdx uint64, approverID flow.Identifier) *flow.ResultApproval {
	return ResultApprovalFixture(
		WithBlockID(result.BlockID),
		WithExecutionResultID(result.ID()),
		WithApproverID(approverID),
		WithChunk(chunkIdx),
	)
}

func EntityWithID(expectedID flow.Identifier) interface{} {
	return mock.MatchedBy(
		func(entity flow.Entity) bool {
			return expectedID == entity.ID()
		})
}

// subgraphFixture represents a subgraph of the blockchain:
//  Result   -----------------------------------> Block
//    |                                             |
//    |                                             v
//    |                                           ParentBlock
//    v
//  PreviousResult  ---> PreviousResult.BlockID
//
// Depending on validity of the subgraph:
//   *  valid:   PreviousResult.BlockID == ParentBlock.ID()
//   *  invalid: PreviousResult.BlockID != ParentBlock.ID()
type subgraphFixture struct {
	Block              *flow.Block
	ParentBlock        *flow.Block
	Result             *flow.ExecutionResult
	PreviousResult     *flow.ExecutionResult
	IncorporatedResult *flow.IncorporatedResult
	Assignment         *chunks.Assignment
	Approvals          map[uint64]map[flow.Identifier]*flow.ResultApproval // chunkIndex -> Verifier Node ID -> Approval
}

// Generates a valid subgraph:
// let
//  * R1 be a result which pertains to blockA
//  * R2 be R1's previous result,
//    where R2 pertains to blockB
// The execution results form a valid subgraph if and only if:
//    blockA.ParentID == blockB.ID
func (bc *BaseChainSuite) ValidSubgraphFixture() subgraphFixture {
	// BLOCKS: <- previousBlock <- block
	parentBlock := BlockFixture()
	block := BlockWithParentFixture(parentBlock.Header)

	// RESULTS for Blocks:
	previousResult := ExecutionResultFixture(WithBlock(&parentBlock))
	result := ExecutionResultFixture(
		WithBlock(&block),
		WithPreviousResult(*previousResult),
	)

	// Exec Receipt for block with valid subgraph
	incorporatedResult := IncorporatedResult.Fixture(IncorporatedResult.WithResult(result))

	// assign each chunk to 50% of validation Nodes and generate respective approvals
	assignment := chunks.NewAssignment()
	assignedVerifiersPerChunk := uint(len(bc.Approvers) / 2)
	approvals := make(map[uint64]map[flow.Identifier]*flow.ResultApproval)
	for _, chunk := range incorporatedResult.Result.Chunks {
		assignedVerifiers := bc.Approvers.Sample(assignedVerifiersPerChunk)
		assignment.Add(chunk, assignedVerifiers.NodeIDs())

		// generate approvals
		chunkApprovals := make(map[flow.Identifier]*flow.ResultApproval)
		for _, approver := range assignedVerifiers {
			chunkApprovals[approver.NodeID] = ApprovalFor(incorporatedResult.Result, chunk.Index, approver.NodeID)
		}
		approvals[chunk.Index] = chunkApprovals
	}

	return subgraphFixture{
		Block:              &block,
		ParentBlock:        &parentBlock,
		Result:             result,
		PreviousResult:     previousResult,
		IncorporatedResult: incorporatedResult,
		Assignment:         assignment,
		Approvals:          approvals,
	}
}

func (bc *BaseChainSuite) Extend(block *flow.Block) {
	bc.Blocks[block.ID()] = block
	bc.SealsIndex[block.ID()] = bc.SealsIndex[block.Header.ParentID]

	for _, receipt := range block.Payload.Receipts {
		// Exec Receipt for block with valid subgraph
		// ATTENTION:
		// Here, IncorporatedBlockID (the first argument) should be set
		// to ancestorID, because that is the block that contains the
		// ExecutionResult. However, in phase 2 of the sealing roadmap,
		// we are still using a temporary sealing logic where the
		// IncorporatedBlockID is expected to be the result's block ID.
		incorporatedResult := IncorporatedResult.Fixture(IncorporatedResult.WithResult(&receipt.ExecutionResult),
			IncorporatedResult.WithIncorporatedBlockID(receipt.ExecutionResult.BlockID))

		// assign each chunk to 50% of validation Nodes and generate respective approvals
		assignment := chunks.NewAssignment()
		assignedVerifiersPerChunk := uint(len(bc.Approvers) / 2)
		approvals := make(map[uint64]map[flow.Identifier]*flow.ResultApproval)
		for _, chunk := range incorporatedResult.Result.Chunks {
			assignedVerifiers := bc.Approvers.Sample(assignedVerifiersPerChunk)
			assignment.Add(chunk, assignedVerifiers.NodeIDs())

			// generate approvals
			chunkApprovals := make(map[flow.Identifier]*flow.ResultApproval)
			for _, approver := range assignedVerifiers {
				chunkApprovals[approver.NodeID] = ApprovalFor(incorporatedResult.Result, chunk.Index, approver.NodeID)
			}
			approvals[chunk.Index] = chunkApprovals
		}

		bc.Assigner.On("Assign", incorporatedResult.Result, incorporatedResult.IncorporatedBlockID).Return(assignment, nil).Maybe()
		bc.PendingApprovals[incorporatedResult.Result.ID()] = approvals
		bc.PendingResults[incorporatedResult.Result.ID()] = incorporatedResult
		bc.Assignments[incorporatedResult.Result.ID()] = assignment
		bc.PersistedResults[receipt.ExecutionResult.ID()] = &receipt.ExecutionResult
		// TODO: adding receipt
	}
}

// addSubgraphFixtureToMempools adds add entities in subgraph to mempools and persistent storage mocks
func (bc *BaseChainSuite) AddSubgraphFixtureToMempools(subgraph subgraphFixture) {
	bc.Blocks[subgraph.ParentBlock.ID()] = subgraph.ParentBlock
	bc.Blocks[subgraph.Block.ID()] = subgraph.Block
	bc.PersistedResults[subgraph.PreviousResult.ID()] = subgraph.PreviousResult
	bc.PersistedResults[subgraph.Result.ID()] = subgraph.Result
	bc.PendingResults[subgraph.IncorporatedResult.ID()] = subgraph.IncorporatedResult

	bc.Assigner.On("Assign", subgraph.IncorporatedResult.Result, subgraph.IncorporatedResult.IncorporatedBlockID).Return(subgraph.Assignment, nil).Maybe()
	bc.PendingApprovals[subgraph.IncorporatedResult.Result.ID()] = subgraph.Approvals
}
