package unittest

import (
	crand "crypto/rand"
	"fmt"
	"math/rand"
	"time"

	"github.com/dapperlabs/flow-go/crypto"
	"github.com/dapperlabs/flow-go/crypto/hash"
	"github.com/dapperlabs/flow-go/engine/verification"
	"github.com/dapperlabs/flow-go/model/cluster"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/messages"
	"github.com/dapperlabs/flow-go/module/mempool/entity"
	"github.com/dapperlabs/flow-go/utils/dsl"
)

func AddressFixture() flow.Address {
	return flow.RootAddress
}

func TransactionSignatureFixture() flow.TransactionSignature {
	return flow.TransactionSignature{
		Address:     AddressFixture(),
		SignerIndex: 0,
		Signature:   []byte{1, 2, 3, 4},
		KeyID:       1,
	}
}

func ProposalKeyFixture() flow.ProposalKey {
	return flow.ProposalKey{
		Address:        AddressFixture(),
		KeyID:          1,
		SequenceNumber: 0,
	}
}

// AccountKeyFixture returns a randomly generated ECDSA/SHA3 account key.
func AccountKeyFixture() (*flow.AccountPrivateKey, error) {
	seed := make([]byte, crypto.KeyGenSeedMinLenECDSAP256)
	_, err := crand.Read(seed)
	if err != nil {
		return nil, err
	}
	key, err := crypto.GeneratePrivateKey(crypto.ECDSAP256, seed)
	if err != nil {
		return nil, err
	}

	return &flow.AccountPrivateKey{
		PrivateKey: key,
		SignAlgo:   key.Algorithm(),
		HashAlgo:   hash.SHA3_256,
	}, nil
}

func BlockFixture() flow.Block {
	header := BlockHeaderFixture()
	return BlockWithParentFixture(&header)
}

func ProposalFixture() *messages.BlockProposal {
	block := BlockFixture()
	return ProposalFromBlock(&block)
}

func ProposalFromBlock(block *flow.Block) *messages.BlockProposal {
	proposal := &messages.BlockProposal{
		Header:  &block.Header,
		Payload: &block.Payload,
	}
	return proposal
}

func PendingFromBlock(block *flow.Block) *flow.PendingBlock {
	pending := flow.PendingBlock{
		OriginID: block.Header.ProposerID,
		Header:   &block.Header,
		Payload:  &block.Payload,
	}
	return &pending
}

func StateDeltaFixture() *messages.ExecutionStateDelta {
	header := BlockHeaderFixture()
	block := BlockWithParentFixture(&header)
	return &messages.ExecutionStateDelta{
		Block: &block,
	}
}

func BlockWithParentFixture(parent *flow.Header) flow.Block {
	payload := flow.Payload{
		Identities: nil,
		Guarantees: CollectionGuaranteesFixture(16),
		Seals:      BlockSealsFixture(16),
	}
	header := BlockHeaderWithParentFixture(parent)
	header.PayloadHash = payload.Hash()
	return flow.Block{
		Header:  header,
		Payload: payload,
	}
}

func StateDeltaWithParentFixture(parent *flow.Header) *messages.ExecutionStateDelta {
	payload := flow.Payload{
		Identities: IdentityListFixture(32),
		Guarantees: CollectionGuaranteesFixture(16),
	}
	header := BlockHeaderWithParentFixture(parent)
	header.PayloadHash = payload.Hash()
	block := flow.Block{
		Header:  header,
		Payload: payload,
	}
	return &messages.ExecutionStateDelta{
		Block: &block,
	}
}

func BlockHeaderFixture() flow.Header {
	return BlockHeaderWithParentFixture(&flow.Header{
		ChainID:  "testing",
		ParentID: IdentifierFixture(),
		Height:   rand.Uint64(),
	})
}

func BlockHeaderWithParentFixture(parent *flow.Header) flow.Header {
	return flow.Header{
		ChainID:        parent.ChainID,
		ParentID:       parent.ID(),
		View:           rand.Uint64(),
		Height:         parent.Height + 1,
		ParentVoterIDs: IdentifierListFixture(4),
		ParentVoterSig: SignatureFixture(),
		ProposerID:     IdentifierFixture(),
		ProposerSig:    SignatureFixture(),
	}
}

func ClusterPayloadFixture(n int) cluster.Payload {
	transactions := make([]*flow.TransactionBody, n)
	for i := 0; i < n; i++ {
		tx := TransactionBodyFixture()
		transactions[i] = &tx
	}
	return cluster.PayloadFromTransactions(transactions...)
}

func ClusterBlockFixture() cluster.Block {

	payload := ClusterPayloadFixture(3)
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

	payload := ClusterPayloadFixture(3)

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
		SignerIDs:    IdentifierListFixture(16),
		Signature:    SignatureFixture(),
	}
}

func CollectionGuaranteesFixture(n int) []*flow.CollectionGuarantee {
	ret := make([]*flow.CollectionGuarantee, 0, n)
	for i := 1; i <= n; i++ {
		cg := flow.CollectionGuarantee{
			CollectionID: IdentifierFixture(),
			Signature:    SignatureFixture(),
		}
		ret = append(ret, &cg)
	}
	return ret
}

func BlockSealFixture() *flow.Seal {
	return &flow.Seal{
		BlockID:      IdentifierFixture(),
		ResultID:     IdentifierFixture(),
		InitialState: StateCommitmentFixture(),
		FinalState:   StateCommitmentFixture(),
	}
}

func BlockSealsFixture(n int) []*flow.Seal {
	seals := make([]*flow.Seal, 0, n)
	for i := 0; i < n; i++ {
		seal := BlockSealFixture()
		if i > 0 {
			seal.InitialState = seals[i-1].FinalState
		}
		seals = append(seals, seal)
	}
	return seals
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
			Signature:    SignatureFixture(),
		},
		Transactions: []*flow.TransactionBody{&txBody},
	}
}

func ExecutableBlockFixture(collectionsSignerIDs [][]flow.Identifier) *entity.ExecutableBlock {

	header := BlockHeaderFixture()
	return ExecutableBlockFixtureWithParent(collectionsSignerIDs, &header)
}

func ExecutableBlockFixtureWithParent(collectionsSignerIDs [][]flow.Identifier, parent *flow.Header) *entity.ExecutableBlock {

	completeCollections := make(map[flow.Identifier]*entity.CompleteCollection, len(collectionsSignerIDs))
	block := BlockWithParentFixture(parent)
	block.Guarantees = nil

	for _, signerIDs := range collectionsSignerIDs {
		completeCollection := CompleteCollectionFixture()
		completeCollection.Guarantee.SignerIDs = signerIDs
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
		ra.Body.ExecutionResultID = id
	}
}

func ResultApprovalFixture(opts ...func(*flow.ResultApproval)) *flow.ResultApproval {
	attestation := flow.Attestation{
		BlockID:           IdentifierFixture(),
		ExecutionResultID: IdentifierFixture(),
		ChunkIndex:        uint64(0),
	}

	approval := flow.ResultApproval{
		Body: flow.ResultApprovalBody{
			Attestation:          attestation,
			ApproverID:           IdentifierFixture(),
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
	_, _ = crand.Read(state[0:20])
	return state
}

func HashFixture(size int) hash.Hash {
	hash := make(hash.Hash, size)
	for i := 0; i < size; i++ {
		hash[i] = byte(i)
	}
	return hash
}

func IdentifierListFixture(n int) []flow.Identifier {
	list := make([]flow.Identifier, n)
	for i := 0; i < n; i++ {
		list[i] = IdentifierFixture()
	}
	return list
}

func IdentifierFixture() flow.Identifier {
	var id flow.Identifier
	_, _ = crand.Read(id[:])
	return id
}

// WithRole adds a role to an identity fixture.
func WithRole(role flow.Role) func(*flow.Identity) {
	return func(identity *flow.Identity) {
		identity.Role = role
	}
}

func WithStake(stake uint64) func(*flow.Identity) {
	return func(identity *flow.Identity) {
		identity.Stake = stake
	}
}

func generateRandomSeed() []byte {
	seed := make([]byte, 48)
	if n, err := crand.Read(seed); err != nil || n != 48 {
		panic(err)
	}
	return seed
}

// IdentityFixture returns a node identity.
func IdentityFixture(opts ...func(*flow.Identity)) *flow.Identity {
	nodeID := IdentifierFixture()
	identity := flow.Identity{
		NodeID:  nodeID,
		Address: fmt.Sprintf("address-%v", nodeID[0:7]),
		Role:    flow.RoleConsensus,
		Stake:   1000,
	}
	for _, apply := range opts {
		apply(&identity)
	}
	return &identity
}

// WithNodeID adds a node ID with the given first byte to an identity.
func WithNodeID(b byte) func(*flow.Identity) {
	return func(identity *flow.Identity) {
		identity.NodeID = flow.Identifier{b}
	}
}

// WithRandomPublicKeys adds random public keys to an identity.
func WithRandomPublicKeys() func(*flow.Identity) {
	return func(identity *flow.Identity) {
		stak, err := crypto.GeneratePrivateKey(crypto.BLSBLS12381, generateRandomSeed())
		if err != nil {
			panic(err)
		}
		identity.StakingPubKey = stak.PublicKey()
		netw, err := crypto.GeneratePrivateKey(crypto.ECDSAP256, generateRandomSeed())
		if err != nil {
			panic(err)
		}
		identity.NetworkPubKey = netw.PublicKey()
	}
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
	_, _ = crand.Read(sig)
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
		Script:             []byte("pub fun main() {}"),
		ReferenceBlockID:   IdentifierFixture(),
		GasLimit:           10,
		ProposalKey:        ProposalKeyFixture(),
		Payer:              AddressFixture(),
		Authorizers:        []flow.Address{AddressFixture()},
		PayloadSignatures:  []flow.TransactionSignature{TransactionSignatureFixture()},
		EnvelopeSignatures: []flow.TransactionSignature{TransactionSignatureFixture()},
	}

	for _, apply := range opts {
		apply(&tb)
	}

	return tb
}

func WithTransactionDSL(txDSL dsl.Transaction) func(tx *flow.TransactionBody) {
	return func(tx *flow.TransactionBody) {
		tx.Script = []byte(txDSL.ToCadence())
	}
}

func TransactionDSLFixture() dsl.Transaction {
	return dsl.Transaction{
		Import: dsl.Import{Address: flow.RootAddress},
		Content: dsl.Prepare{
			Content: dsl.Code(`
				pub fun main() {}
			`),
		},
	}
}

// VerifiableChunk returns a complete verifiable chunk with an
// execution receipt referencing the block/collections.
func VerifiableChunkFixture(chunkIndex uint64) *verification.VerifiableChunk {

	guarantees := make([]*flow.CollectionGuarantee, 0)

	var col flow.Collection

	for i := 0; i <= int(chunkIndex); i++ {
		col = CollectionFixture(1)
		guarantee := col.Guarantee()
		guarantees = append(guarantees, &guarantee)
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

	chunks := make([]*flow.Chunk, 0)

	var chunk flow.Chunk

	for i := 0; i <= int(chunkIndex); i++ {
		chunk = flow.Chunk{
			ChunkBody: flow.ChunkBody{
				CollectionIndex: uint(i),
				StartState:      StateCommitmentFixture(),
			},
			Index: uint64(i),
		}
		chunks = append(chunks, &chunk)
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

	return &verification.VerifiableChunk{
		ChunkIndex: chunkIndex,
		EndState:   StateCommitmentFixture(),
		Block:      &block,
		Receipt:    &receipt,
		Collection: &col,
	}
}

func ChunkDataPackFixture(identifier flow.Identifier) flow.ChunkDataPack {
	return flow.ChunkDataPack{
		ChunkID:         identifier,
		StartState:      StateCommitmentFixture(),
		RegisterTouches: []flow.RegisterTouch{flow.RegisterTouch{RegisterID: []byte{'1'}, Value: []byte{'a'}, Proof: []byte{'p'}}},
	}
}

// SeedFixture returns a random []byte with length n
func SeedFixture(n int) []byte {
	var seed = make([]byte, n)
	_, _ = crand.Read(seed[0:n])
	return seed
}

// SeedFixtures returns a list of m random []byte, each having length n
func SeedFixtures(m int, n int) [][]byte {
	var seeds = make([][]byte, m, n)
	for i := range seeds {
		seeds[i] = SeedFixture(n)
	}
	return seeds
}

// EventFixture returns an event
func EventFixture(eType flow.EventType, transactionIndex uint32, eventIndex uint32, txID flow.Identifier) flow.Event {
	return flow.Event{
		Type:             eType,
		TransactionIndex: transactionIndex,
		EventIndex:       eventIndex,
		Payload:          []byte{},
		TransactionID:    txID,
	}
}

func EmulatorRootKey() (*flow.AccountPrivateKey, error) {

	// TODO seems this key literal doesn't decode anymore
	emulatorRootKey, err := crypto.DecodePrivateKey(crypto.ECDSAP256, []byte("f87db87930770201010420ae2cc975dcbdd0ebc56f268b1d8a95834c2955970aea27042d35ec9f298b9e5aa00a06082a8648ce3d030107a1440342000417f5a527137785d2d773fee84b4c7ee40266a1dd1f36ddd46ecf25db6df6a499459629174de83256f2a44ebd4325b9def67d523b755a8926218c4efb7904f8ce0203"))
	if err != nil {
		return nil, err
	}

	return &flow.AccountPrivateKey{
		PrivateKey: emulatorRootKey,
		SignAlgo:   emulatorRootKey.Algorithm(),
		HashAlgo:   hash.SHA3_256,
	}, nil
}

// NoopTxScript returns a Cadence script for a no-op transaction.
func NoopTxScript() []byte {
	return []byte("transaction {}")
}
