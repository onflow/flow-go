package unittest

import (
	"fmt"
	"math/rand"
	"time"

	"github.com/dapperlabs/flow-go/crypto"
	"github.com/dapperlabs/flow-go/engine/verification"
	"github.com/dapperlabs/flow-go/model/cluster"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/module/mempool/entity"
)

func AddressFixture() flow.Address {
	return flow.RootAddress
}

func AccountSignatureFixture() flow.AccountSignature {
	return flow.AccountSignature{
		Account:   AddressFixture(),
		Signature: []byte{1, 2, 3, 4},
	}
}

func BlockFixture() flow.Block {
	header := BlockHeaderFixture()
	return BlockWithParentFixture(&header)
}

func BlockWithParentFixture(parent *flow.Header) flow.Block {
	payload := flow.Payload{
		Identities: IdentityListFixture(32),
		Guarantees: CollectionGuaranteesFixture(16),
	}
	header := BlockHeaderWithParentFixture(parent)
	header.PayloadHash = payload.Hash()
	return flow.Block{
		Header:  header,
		Payload: payload,
	}
}

func BlockHeaderFixture() flow.Header {
	return BlockHeaderWithParentFixture(&flow.Header{
		ParentID: IdentifierFixture(),
		Height:   0,
	})
}

func BlockHeaderWithParentFixture(parent *flow.Header) flow.Header {
	return flow.Header{
		ParentID: parent.ID(),
		View:     rand.Uint64(),
		Height:   parent.Height + 1,
	}
}

// BlockWithParent creates a new block that is valid
// with respect to the given parent block.
func BlockWithParent(parent *flow.Block) flow.Block {
	payload := flow.Payload{
		Identities: IdentityListFixture(32),
		Guarantees: CollectionGuaranteesFixture(16),
	}

	header := BlockHeaderFixture()
	header.View = parent.View + 1
	header.ChainID = parent.ChainID
	header.Timestamp = time.Now()
	header.ParentID = parent.ID()
	header.PayloadHash = payload.Hash()

	return flow.Block{
		Header:  header,
		Payload: payload,
	}
}

func SealFixture() flow.Seal {
	return flow.Seal{
		BlockID:       IdentifierFixture(),
		PreviousState: StateCommitmentFixture(),
		FinalState:    StateCommitmentFixture(),
		Signature:     SignatureFixture(),
	}
}

func ClusterBlockFixture() cluster.Block {
	payload := cluster.Payload{
		Collection: flow.LightCollection{
			Transactions: []flow.Identifier{IdentifierFixture()},
		},
	}
	header := BlockHeaderFixture()
	header.PayloadHash = payload.Hash()
	return cluster.Block{
		Header:  header,
		Payload: payload,
	}
}

// ClusterBlockWithParent creates a new cluster consensus block that is valid
// with respect to the given parent block.
func ClusterBlockWithParent(parent *cluster.Block) cluster.Block {
	payload := cluster.Payload{
		Collection: flow.LightCollection{
			Transactions: []flow.Identifier{IdentifierFixture()},
		},
	}

	header := BlockHeaderFixture()
	header.Height = parent.Height + 1
	header.View = parent.View + 1
	header.ChainID = parent.ChainID
	header.Timestamp = time.Now()
	header.ParentID = parent.ID()
	header.PayloadHash = payload.Hash()

	block := cluster.Block{
		Header:  header,
		Payload: payload,
	}

	return block
}

func CollectionGuaranteeFixture() *flow.CollectionGuarantee {
	return &flow.CollectionGuarantee{
		CollectionID: IdentifierFixture(),
		Signatures:   SignaturesFixture(16),
	}
}

func CollectionGuaranteesFixture(n int) []*flow.CollectionGuarantee {
	ret := make([]*flow.CollectionGuarantee, 0, n)
	for i := 1; i <= n; i++ {
		cg := flow.CollectionGuarantee{
			CollectionID: flow.Identifier{byte(i)},
			Signatures:   []crypto.Signature{[]byte(fmt.Sprintf("signature %d A", i)), []byte(fmt.Sprintf("signature %d B", i))},
		}
		ret = append(ret, &cg)
	}
	return ret
}

func CollectionFixture(n int) flow.Collection {
	transactions := make([]*flow.TransactionBody, 0, n)

	for i := 0; i < n; i++ {
		tx := TransactionFixture()
		transactions = append(transactions, &tx.TransactionBody)
	}

	return flow.Collection{Transactions: transactions}
}

func ExecutionReceiptFixture() *flow.ExecutionReceipt {
	return &flow.ExecutionReceipt{
		ExecutorID:        IdentifierFixture(),
		ExecutionResult:   *ExecutionResultFixture(),
		Spocks:            nil,
		ExecutorSignature: SignatureFixture(),
	}
}

func CompleteCollectionFixture() *entity.CompleteCollection {
	txBody := TransactionBodyFixture()
	return &entity.CompleteCollection{
		Guarantee: &flow.CollectionGuarantee{
			CollectionID: flow.Collection{Transactions: []*flow.TransactionBody{&txBody}}.ID(),
			Signatures:   SignaturesFixture(16),
		},
		Transactions: []*flow.TransactionBody{&txBody},
	}
}

func ExecutableBlockFixture(collections int) *entity.ExecutableBlock {

	header := BlockHeaderFixture()
	return ExecutableBlockFixtureWithParent(collections, &header)
}

func ExecutableBlockFixtureWithParent(collections int, parent *flow.Header) *entity.ExecutableBlock {

	completeCollections := make(map[flow.Identifier]*entity.CompleteCollection, collections)
	block := BlockWithParentFixture(parent)
	block.Guarantees = nil

	for i := 0; i < collections; i++ {
		completeCollection := CompleteCollectionFixture()
		block.Guarantees = append(block.Guarantees, completeCollection.Guarantee)
		completeCollections[completeCollection.Guarantee.CollectionID] = completeCollection
	}

	block.PayloadHash = block.Payload.Hash()

	return &entity.ExecutableBlock{
		Block:               &block,
		CompleteCollections: completeCollections,
	}
}

func ExecutionResultFixture() *flow.ExecutionResult {
	return &flow.ExecutionResult{
		ExecutionResultBody: flow.ExecutionResultBody{
			PreviousResultID: IdentifierFixture(),
			BlockID:          IdentifierFixture(),
			FinalStateCommit: StateCommitmentFixture(),
			Chunks: flow.ChunkList{
				ChunkFixture(),
				ChunkFixture(),
			},
		},
		Signatures: SignaturesFixture(6),
	}
}

func WithExecutionResultID(id flow.Identifier) func(*flow.ResultApproval) {
	return func(ra *flow.ResultApproval) {
		ra.ResultApprovalBody.ExecutionResultID = id
	}
}

func ResultApprovalFixture(opts ...func(*flow.ResultApproval)) *flow.ResultApproval {
	approval := flow.ResultApproval{
		ResultApprovalBody: flow.ResultApprovalBody{
			ExecutionResultID:    IdentifierFixture(),
			AttestationSignature: SignatureFixture(),
			Spock:                nil,
		},
		VerifierSignature: SignatureFixture(),
	}

	for _, apply := range opts {
		apply(&approval)
	}

	return &approval
}

func StateCommitmentFixture() flow.StateCommitment {
	var state = make([]byte, 20)
	_, _ = rand.Read(state[0:20])
	return state
}

func HashFixture(size int) crypto.Hash {
	hash := make(crypto.Hash, size)
	for i := 0; i < size; i++ {
		hash[i] = byte(i)
	}
	return hash
}

func IdentifierFixture() flow.Identifier {
	var id flow.Identifier
	_, _ = rand.Read(id[:])
	return id
}

// WithRole adds a role to an identity fixture.
func WithRole(role flow.Role) func(*flow.Identity) {
	return func(id *flow.Identity) {
		id.Role = role
	}
}

// IdentityFixture returns a node identity.
func IdentityFixture(opts ...func(*flow.Identity)) *flow.Identity {
	id := flow.Identity{
		NodeID:  IdentifierFixture(),
		Address: "address",
		Role:    flow.RoleConsensus,
		Stake:   1000,
	}
	for _, apply := range opts {
		apply(&id)
	}
	return &id
}

// IdentityListFixture returns a list of node identity objects. The identities
// can be customized (ie. set their role) by passing in a function that modifies
// the input identities as required.
func IdentityListFixture(n int, opts ...func(*flow.Identity)) flow.IdentityList {
	identities := make(flow.IdentityList, n)

	for i := 0; i < n; i++ {
		identity := IdentityFixture()
		identity.Address = fmt.Sprintf("address-%d", i+1)
		for _, opt := range opts {
			opt(identity)
		}
		identities[i] = identity
	}

	return identities
}

func ChunkFixture() *flow.Chunk {
	return &flow.Chunk{
		ChunkBody: flow.ChunkBody{
			CollectionIndex:      42,
			StartState:           StateCommitmentFixture(),
			EventCollection:      IdentifierFixture(),
			TotalComputationUsed: 4200,
			NumberOfTransactions: 42,
		},
		Index:    0,
		EndState: StateCommitmentFixture(),
	}
}

func SignatureFixture() crypto.Signature {
	sig := make([]byte, 32)
	_, _ = rand.Read(sig)
	return sig
}

func SignaturesFixture(n int) []crypto.Signature {
	var sigs []crypto.Signature
	for i := 0; i < n; i++ {
		sigs = append(sigs, SignatureFixture())
	}
	return sigs
}

func TransactionFixture(n ...func(t *flow.Transaction)) flow.Transaction {
	tx := flow.Transaction{TransactionBody: TransactionBodyFixture()}
	if len(n) > 0 {
		n[0](&tx)
	}
	return tx
}

func TransactionBodyFixture(opts ...func(*flow.TransactionBody)) flow.TransactionBody {
	tb := flow.TransactionBody{
		Script:           []byte("pub fun main() {}"),
		ReferenceBlockID: IdentifierFixture(),
		Nonce:            rand.Uint64(),
		ComputeLimit:     10,
		PayerAccount:     AddressFixture(),
		ScriptAccounts:   []flow.Address{AddressFixture()},
		Signatures:       []flow.AccountSignature{AccountSignatureFixture()},
	}

	for _, apply := range opts {
		apply(&tb)
	}

	return tb
}

// CompleteExecutionResultFixture returns complete execution result with an
// execution receipt referencing the block/collections.
// chunkCount determines the number of chunks inside each receipt
func CompleteExecutionResultFixture(chunkCount int) verification.CompleteExecutionResult {
	chunks := make([]*flow.Chunk, 0)
	chunkStates := make([]*flow.ChunkState, 0, chunkCount)
	collections := make([]*flow.Collection, 0, chunkCount)
	guarantees := make([]*flow.CollectionGuarantee, 0, chunkCount)
	chunkDataPacks := make([]*flow.ChunkDataPack, 0, chunkCount)

	for i := 0; i < chunkCount; i++ {
		// creates one guaranteed collection per chunk
		coll := CollectionFixture(3)
		guarantee := coll.Guarantee()
		collections = append(collections, &coll)
		guarantees = append(guarantees, &guarantee)

		// creates a chunk
		chunk := &flow.Chunk{
			ChunkBody: flow.ChunkBody{
				CollectionIndex: uint(i),
				StartState:      StateCommitmentFixture(),
			},
			Index: uint64(i),
		}
		chunks = append(chunks, chunk)

		// creates a chunk state
		chunkState := &flow.ChunkState{
			ChunkID:   chunk.ID(),
			Registers: flow.Ledger{},
		}
		chunkStates = append(chunkStates, chunkState)

		// creates a chunk data pack for the chunk
		chunkDataPack := ChunkDataPackFixture(chunk.ID())
		chunkDataPacks = append(chunkDataPacks, &chunkDataPack)
	}

	payload := flow.Payload{
		Identities: IdentityListFixture(32),
		Guarantees: guarantees,
	}
	header := BlockHeaderFixture()
	header.PayloadHash = payload.Hash()

	block := flow.Block{
		Header:  header,
		Payload: payload,
	}

	result := flow.ExecutionResult{
		ExecutionResultBody: flow.ExecutionResultBody{
			BlockID: block.ID(),
			Chunks:  chunks,
		},
	}

	receipt := flow.ExecutionReceipt{
		ExecutionResult: result,
	}

	return verification.CompleteExecutionResult{
		Receipt:        &receipt,
		Block:          &block,
		Collections:    collections,
		ChunkStates:    chunkStates,
		ChunkDataPacks: chunkDataPacks,
	}
}

// VerifiableChunk returns a complete verifiable chunk with an
// execution receipt referencing the block/collections.
func VerifiableChunkFixture() *verification.VerifiableChunk {
	coll := CollectionFixture(3)
	guarantee := coll.Guarantee()

	payload := flow.Payload{
		Identities: IdentityListFixture(32),
		Guarantees: []*flow.CollectionGuarantee{&guarantee},
	}
	header := BlockHeaderFixture()
	header.PayloadHash = payload.Hash()

	block := flow.Block{
		Header:  header,
		Payload: payload,
	}

	chunk := flow.Chunk{
		ChunkBody: flow.ChunkBody{
			CollectionIndex: 0,
			StartState:      StateCommitmentFixture(),
		},
		Index: 0,
	}

	chunkState := flow.ChunkState{
		ChunkID:   chunk.ID(),
		Registers: flow.Ledger{},
	}

	result := flow.ExecutionResult{
		ExecutionResultBody: flow.ExecutionResultBody{
			BlockID: block.ID(),
			Chunks:  flow.ChunkList{&chunk},
		},
	}

	receipt := flow.ExecutionReceipt{
		ExecutionResult: result,
	}

	return &verification.VerifiableChunk{
		ChunkIndex: chunk.Index,
		EndState:   StateCommitmentFixture(),
		Block:      &block,
		Receipt:    &receipt,
		Collection: &coll,
		ChunkState: &chunkState,
	}
}

func ChunkHeaderFixture() flow.ChunkHeader {
	return flow.ChunkHeader{
		ChunkID:     IdentifierFixture(),
		StartState:  StateCommitmentFixture(),
		RegisterIDs: []flow.RegisterID{{1}, {2}, {3}},
	}
}

func ChunkDataPackFixture(identifier flow.Identifier) flow.ChunkDataPack {
	return flow.ChunkDataPack{
		ChunkID:         identifier,
		StartState:      StateCommitmentFixture(),
		RegisterTouches: []flow.RegisterTouch{flow.RegisterTouch{RegisterID: []byte{'1'}, Value: []byte{'a'}, Proof: []byte{'p'}}},
	}
}
