package unittest

import (
	"fmt"
	"math/rand"
	"time"

	"github.com/dapperlabs/flow-go/crypto"
	"github.com/dapperlabs/flow-go/engine/execution"
	"github.com/dapperlabs/flow-go/engine/execution/state"
	"github.com/dapperlabs/flow-go/engine/verification"
	"github.com/dapperlabs/flow-go/model/cluster"
	"github.com/dapperlabs/flow-go/model/flow"
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
	payload := flow.Payload{
		Identities: IdentityListFixture(32),
		Guarantees: CollectionGuaranteesFixture(16),
	}
	header := BlockHeaderFixture()
	header.PayloadHash = payload.Hash()
	return flow.Block{
		Header:  header,
		Payload: payload,
	}
}

func BlockHeaderFixture() flow.Header {
	return flow.Header{
		ParentID: IdentifierFixture(),
		Number:   rand.Uint64(),
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

	header := flow.Header{
		Number:      parent.Number + 1,
		ChainID:     parent.ChainID,
		Timestamp:   time.Now(),
		ParentID:    parent.ID(),
		PayloadHash: payload.Hash(),
	}

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

func StateViewFixture() *state.View {
	return state.NewView(func(key string) (bytes []byte, err error) {
		return nil, nil
	})
}

func CompleteCollectionFixture() *execution.CompleteCollection {
	txBody := TransactionBodyFixture()
	return &execution.CompleteCollection{
		Guarantee:    CollectionGuaranteeFixture(),
		Transactions: []*flow.TransactionBody{&txBody},
	}
}

func CompleteBlockFixture() *execution.CompleteBlock {

	cc1 := CompleteCollectionFixture()
	cc2 := CompleteCollectionFixture()

	block := BlockFixture()
	return &execution.CompleteBlock{
		Block:               &block,
		CompleteCollections: map[flow.Identifier]*execution.CompleteCollection{cc1.Guarantee.CollectionID: cc1, cc2.Guarantee.CollectionID: cc2},
	}
}

func ComputationResultFixture() *execution.ComputationResult {
	return &execution.ComputationResult{
		CompleteBlock: CompleteBlockFixture(),
		StateViews:    []*state.View{StateViewFixture(), StateViewFixture()},
		StartState:    StateCommitmentFixture(),
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
			ChunkIndexList:       nil,
			Proof:                nil,
			Spocks:               nil,
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
			CollectionIndex:                 42,
			StartState:                      StateCommitmentFixture(),
			EventCollection:                 IdentifierFixture(),
			TotalComputationUsed:            4200,
			FirstTransactionComputationUsed: 42,
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

func TransactionBodyFixture() flow.TransactionBody {
	return flow.TransactionBody{
		Script:           []byte("pub fun main() {}"),
		ReferenceBlockID: IdentifierFixture(),
		Nonce:            rand.Uint64(),
		ComputeLimit:     10,
		PayerAccount:     AddressFixture(),
		ScriptAccounts:   []flow.Address{AddressFixture()},
		Signatures:       []flow.AccountSignature{AccountSignatureFixture()},
	}
}

// CompleteExecutionResultFixture returns complete execution result with an
// execution receipt referencing the block/collections.
func CompleteExecutionResultFixture() verification.CompleteExecutionResult {
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

	return verification.CompleteExecutionResult{
		Receipt:     &receipt,
		Block:       &block,
		Collections: []*flow.Collection{&coll},
		ChunkStates: []*flow.ChunkState{&chunkState},
	}
}

func ChunkHeaderFixture() flow.ChunkHeader {
	return flow.ChunkHeader{
		ChunkID:     IdentifierFixture(),
		StartState:  StateCommitmentFixture(),
		RegisterIDs: []flow.RegisterID{{1}, {2}, {3}},
	}
}
