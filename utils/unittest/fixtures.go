package unittest

import (
	crand "crypto/rand"
	"fmt"
	"math/rand"
	"time"

	sdk "github.com/onflow/flow-go-sdk"

	"github.com/onflow/flow-go/model/chunks"
	"github.com/onflow/flow-go/model/flow/order"
	"github.com/onflow/flow-go/module/signature"
	"github.com/onflow/flow-go/state/protocol/inmem"

	hotstuff "github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/crypto/hash"
	"github.com/onflow/flow-go/engine/execution/state/delta"
	"github.com/onflow/flow-go/engine/verification"
	"github.com/onflow/flow-go/model/cluster"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/model/messages"
	"github.com/onflow/flow-go/module/mempool/entity"
	"github.com/onflow/flow-go/utils/dsl"
)

func AddressFixture() flow.Address {
	return flow.Testnet.Chain().ServiceAddress()
}

func RandomAddressFixture() flow.Address {
	// we use a 32-bit index - since the linear address generator uses 45 bits,
	// this won't error
	addr, err := flow.Testnet.Chain().AddressAtIndex(uint64(rand.Uint32()))
	if err != nil {
		panic(err)
	}
	return addr
}

func InvalidAddressFixture() flow.Address {
	addr := AddressFixture()
	addr[0] ^= 1 // alter one bit to obtain an invalid address
	if flow.Testnet.Chain().IsValid(addr) {
		panic("invalid address fixture generated valid address")
	}
	return addr
}

func InvalidFormatSignature() flow.TransactionSignature {
	return flow.TransactionSignature{
		Address:     AddressFixture(),
		SignerIndex: 0,
		Signature:   make([]byte, crypto.SignatureLenECDSAP256), // zero signature is invalid
		KeyIndex:    1,
	}
}

func TransactionSignatureFixture() flow.TransactionSignature {
	sigLen := crypto.SignatureLenECDSAP256
	s := flow.TransactionSignature{
		Address:     AddressFixture(),
		SignerIndex: 0,
		Signature:   SeedFixture(sigLen),
		KeyIndex:    1,
	}
	// make sure the ECDSA signature passes the format check
	s.Signature[sigLen/2] = 0
	s.Signature[0] = 0
	s.Signature[sigLen/2-1] |= 1
	s.Signature[sigLen-1] |= 1
	return s
}

func ProposalKeyFixture() flow.ProposalKey {
	return flow.ProposalKey{
		Address:        AddressFixture(),
		KeyIndex:       1,
		SequenceNumber: 0,
	}
}

// AccountKeyDefaultFixture returns a randomly generated ECDSA/SHA3 account key.
func AccountKeyDefaultFixture() (*flow.AccountPrivateKey, error) {
	return AccountKeyFixture(crypto.KeyGenSeedMinLenECDSAP256, crypto.ECDSAP256, hash.SHA3_256)
}

// AccountKeyFixture returns a randomly generated account key.
func AccountKeyFixture(
	seedLength int,
	signingAlgo crypto.SigningAlgorithm,
	hashAlgo hash.HashingAlgorithm,
) (*flow.AccountPrivateKey, error) {
	seed := make([]byte, seedLength)

	_, err := crand.Read(seed)
	if err != nil {
		return nil, err
	}

	key, err := crypto.GeneratePrivateKey(signingAlgo, seed)
	if err != nil {
		return nil, err
	}

	return &flow.AccountPrivateKey{
		PrivateKey: key,
		SignAlgo:   key.Algorithm(),
		HashAlgo:   hashAlgo,
	}, nil
}

func BlockFixture() flow.Block {
	header := BlockHeaderFixture()
	return BlockWithParentFixture(&header)
}

func FullBlockFixture() flow.Block {
	block := BlockFixture()
	payload := block.Payload
	payload.Seals = Seal.Fixtures(10)
	payload.Receipts = []*flow.ExecutionReceipt{
		ExecutionReceiptFixture(),
		ExecutionReceiptFixture(),
	}

	header := block.Header
	header.PayloadHash = payload.Hash()

	return flow.Block{
		Header:  header,
		Payload: payload,
	}
}

func BlockFixtures(number int) []*flow.Block {
	blocks := make([]*flow.Block, 0, number)
	for ; number > 0; number-- {
		block := BlockFixture()
		blocks = append(blocks, &block)
	}
	return blocks
}

func ProposalFixture() *messages.BlockProposal {
	block := BlockFixture()
	return ProposalFromBlock(&block)
}

func ProposalFromBlock(block *flow.Block) *messages.BlockProposal {
	proposal := &messages.BlockProposal{
		Header:  block.Header,
		Payload: block.Payload,
	}
	return proposal
}

func PendingFromBlock(block *flow.Block) *flow.PendingBlock {
	pending := flow.PendingBlock{
		OriginID: block.Header.ProposerID,
		Header:   block.Header,
		Payload:  block.Payload,
	}
	return &pending
}

func StateDeltaFixture() *messages.ExecutionStateDelta {
	header := BlockHeaderFixture()
	block := BlockWithParentFixture(&header)
	return &messages.ExecutionStateDelta{
		ExecutableBlock: entity.ExecutableBlock{
			Block: &block,
		},
	}
}

func ReceiptAndSealForBlock(block *flow.Block) (*flow.ExecutionReceipt, *flow.Seal) {
	receipt := ReceiptForBlockFixture(block)
	seal := Seal.Fixture(Seal.WithBlock(block.Header), Seal.WithResult(&receipt.ExecutionResult))
	return receipt, seal
}

func PayloadFixture(options ...func(*flow.Payload)) flow.Payload {
	payload := flow.EmptyPayload()
	for _, option := range options {
		option(&payload)
	}
	return payload
}

// WithAllTheFixins ensures a payload contains no empty slice fields. When
// encoding and decoding, nil vs empty slices are not preserved, which can
// result in two models that are semantically equal being considered non-equal
// by our testing framework.
func WithAllTheFixins(payload *flow.Payload) {
	payload.Seals = Seal.Fixtures(3)
	payload.Guarantees = CollectionGuaranteesFixture(4)
	payload.Receipts = []*flow.ExecutionReceipt{ExecutionReceiptFixture()}
}

func WithSeals(seals ...*flow.Seal) func(*flow.Payload) {
	return func(payload *flow.Payload) {
		payload.Seals = append(payload.Seals, seals...)
	}
}

func WithGuarantees(guarantees ...*flow.CollectionGuarantee) func(*flow.Payload) {
	return func(payload *flow.Payload) {
		payload.Guarantees = append(payload.Guarantees, guarantees...)
	}
}

func WithReceipts(receipts ...*flow.ExecutionReceipt) func(*flow.Payload) {
	return func(payload *flow.Payload) {
		payload.Receipts = append(payload.Receipts, receipts...)
	}
}

func BlockWithParentFixture(parent *flow.Header) flow.Block {
	payload := PayloadFixture()
	header := BlockHeaderWithParentFixture(parent)
	header.PayloadHash = payload.Hash()
	return flow.Block{
		Header:  &header,
		Payload: &payload,
	}
}

//func BlockWithParentFixture(parent *flow.Header) flow.Block {
//	payload := PayloadFixture()
//}

func WithoutGuarantee(payload *flow.Payload) {
	payload.Guarantees = nil
}

//
//func BlockWithParentFixture(parent *flow.Header, options ...func(*flow.Payload)) flow.Block {
//	payload := PayloadFixture(WithoutSeals)
//	header := BlockHeaderWithParentFixture(parent)
//	header.Height = parent.Height + 1
//
//	for _, apply := range options {
//		apply(payload)
//	}
//
//	header.PayloadHash = payload.Hash()
//
//	return flow.Block{
//		Header:  &header,
//		Payload: &payload,
//	}
//}

func StateInteractionsFixture() *delta.Snapshot {
	return &delta.NewView(nil).Interactions().Snapshot
}

func BlockWithParentAndProposerFixture(parent *flow.Header, proposer flow.Identifier) flow.Block {
	block := BlockWithParentFixture(parent)

	block.Header.ProposerID = proposer
	block.Header.ParentVoterIDs = []flow.Identifier{proposer}

	return block
}

func BlockWithParentAndSeal(
	parent *flow.Header, sealed *flow.Header) *flow.Block {
	block := BlockWithParentFixture(parent)
	payload := flow.Payload{
		Guarantees: nil,
	}

	if sealed != nil {
		payload.Seals = []*flow.Seal{
			Seal.Fixture(
				Seal.WithBlockID(sealed.ID()),
			),
		}
	}

	block.SetPayload(payload)
	return &block
}

func StateDeltaWithParentFixture(parent *flow.Header) *messages.ExecutionStateDelta {
	payload := PayloadFixture()
	header := BlockHeaderWithParentFixture(parent)
	header.PayloadHash = payload.Hash()
	block := flow.Block{
		Header:  &header,
		Payload: &payload,
	}

	var stateInteractions []*delta.Snapshot
	stateInteractions = append(stateInteractions, StateInteractionsFixture())

	return &messages.ExecutionStateDelta{
		ExecutableBlock: entity.ExecutableBlock{
			Block: &block,
		},
		StateInteractions: stateInteractions,
	}
}

func GenesisFixture(identities flow.IdentityList) *flow.Block {
	genesis := flow.Genesis(flow.Emulator)
	return genesis
}

func BlockHeaderFixture() flow.Header {
	height := uint64(rand.Uint32())
	view := height + uint64(rand.Intn(1000))
	return BlockHeaderWithParentFixture(&flow.Header{
		ChainID:  flow.Emulator,
		ParentID: IdentifierFixture(),
		Height:   height,
		View:     view,
	})
}

func BlockHeaderFixtureOnChain(chainID flow.ChainID) flow.Header {
	height := uint64(rand.Uint32())
	view := height + uint64(rand.Intn(1000))
	return BlockHeaderWithParentFixture(&flow.Header{
		ChainID:  chainID,
		ParentID: IdentifierFixture(),
		Height:   height,
		View:     view,
	})
}

func BlockHeaderWithParentFixture(parent *flow.Header) flow.Header {
	height := parent.Height + 1
	view := parent.View + 1 + uint64(rand.Intn(10)) // Intn returns [0, n)
	return flow.Header{
		ChainID:        parent.ChainID,
		ParentID:       parent.ID(),
		Height:         height,
		PayloadHash:    IdentifierFixture(),
		Timestamp:      time.Now().UTC(),
		View:           view,
		ParentVoterIDs: IdentifierListFixture(4),
		ParentVoterSig: CombinedSignatureFixture(2),
		ProposerID:     IdentifierFixture(),
		ProposerSig:    SignatureFixture(),
	}
}

func ClusterPayloadFixture(n int) *cluster.Payload {
	transactions := make([]*flow.TransactionBody, n)
	for i := 0; i < n; i++ {
		tx := TransactionBodyFixture()
		transactions[i] = &tx
	}
	payload := cluster.PayloadFromTransactions(flow.ZeroID, transactions...)
	return &payload
}

func ClusterBlockFixture() cluster.Block {

	payload := ClusterPayloadFixture(3)
	header := BlockHeaderFixture()
	header.PayloadHash = payload.Hash()

	return cluster.Block{
		Header:  &header,
		Payload: payload,
	}
}

// ClusterBlockWithParent creates a new cluster consensus block that is valid
// with respect to the given parent block.
func ClusterBlockWithParent(parent *cluster.Block) cluster.Block {

	payload := ClusterPayloadFixture(3)

	header := BlockHeaderFixture()
	header.Height = parent.Header.Height + 1
	header.View = parent.Header.View + 1
	header.ChainID = parent.Header.ChainID
	header.Timestamp = time.Now()
	header.ParentID = parent.ID()
	header.PayloadHash = payload.Hash()

	block := cluster.Block{
		Header:  &header,
		Payload: payload,
	}

	return block
}

func WithCollRef(refID flow.Identifier) func(*flow.CollectionGuarantee) {
	return func(guarantee *flow.CollectionGuarantee) {
		guarantee.ReferenceBlockID = refID
	}
}

func WithCollection(collection *flow.Collection) func(guarantee *flow.CollectionGuarantee) {
	return func(guarantee *flow.CollectionGuarantee) {
		guarantee.CollectionID = collection.ID()
	}
}

func CollectionGuaranteeFixture(options ...func(*flow.CollectionGuarantee)) *flow.CollectionGuarantee {
	guarantee := &flow.CollectionGuarantee{
		CollectionID: IdentifierFixture(),
		SignerIDs:    IdentifierListFixture(16),
		Signature:    SignatureFixture(),
	}
	for _, option := range options {
		option(guarantee)
	}
	return guarantee
}

func CollectionGuaranteesFixture(n int, options ...func(*flow.CollectionGuarantee)) []*flow.CollectionGuarantee {
	guarantees := make([]*flow.CollectionGuarantee, 0, n)
	for i := 1; i <= n; i++ {
		guarantee := CollectionGuaranteeFixture(options...)
		guarantees = append(guarantees, guarantee)
	}
	return guarantees
}

func BlockSealsFixture(n int) []*flow.Seal {
	seals := make([]*flow.Seal, 0, n)
	for i := 0; i < n; i++ {
		seal := Seal.Fixture()
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
	block.Payload.Guarantees = nil

	for _, signerIDs := range collectionsSignerIDs {
		completeCollection := CompleteCollectionFixture()
		completeCollection.Guarantee.SignerIDs = signerIDs
		block.Payload.Guarantees = append(block.Payload.Guarantees, completeCollection.Guarantee)
		completeCollections[completeCollection.Guarantee.CollectionID] = completeCollection
	}

	block.Header.PayloadHash = block.Payload.Hash()

	executableBlock := &entity.ExecutableBlock{
		Block:               &block,
		CompleteCollections: completeCollections,
	}
	// Preload the id
	executableBlock.ID()
	return executableBlock
}

func WithExecutorID(executorID flow.Identifier) func(*flow.ExecutionReceipt) {
	return func(er *flow.ExecutionReceipt) {
		er.ExecutorID = executorID
	}
}

func WithResult(result *flow.ExecutionResult) func(*flow.ExecutionReceipt) {
	return func(receipt *flow.ExecutionReceipt) {
		receipt.ExecutionResult = *result
	}
}

func ExecutionReceiptFixture(opts ...func(*flow.ExecutionReceipt)) *flow.ExecutionReceipt {
	receipt := &flow.ExecutionReceipt{
		ExecutorID:        IdentifierFixture(),
		ExecutionResult:   *ExecutionResultFixture(),
		Spocks:            nil,
		ExecutorSignature: SignatureFixture(),
	}

	for _, apply := range opts {
		apply(receipt)
	}

	return receipt
}

func ReceiptForBlockFixture(block *flow.Block) *flow.ExecutionReceipt {
	return ReceiptForBlockExecutorFixture(block, IdentifierFixture())
}

func ReceiptForBlockExecutorFixture(block *flow.Block, executor flow.Identifier) *flow.ExecutionReceipt {
	result := ExecutionResultFixture(WithBlock(block))
	receipt := ExecutionReceiptFixture(WithResult(result), WithExecutorID(executor))
	return receipt
}

func WithPreviousResult(prevResult flow.ExecutionResult) func(*flow.ExecutionResult) {
	return func(result *flow.ExecutionResult) {
		result.PreviousResultID = prevResult.ID()
		finalState, ok := prevResult.FinalStateCommitment()
		if !ok {
			panic("missing final state commitment")
		}
		result.Chunks[0].StartState = finalState
	}
}

func WithBlock(block *flow.Block) func(*flow.ExecutionResult) {
	chunks := 1 // tailing chunk is always system chunk
	var previousResultID flow.Identifier
	if block.Payload != nil {
		chunks += len(block.Payload.Guarantees)
	}
	blockID := block.ID()

	return func(result *flow.ExecutionResult) {
		startState := result.Chunks[0].StartState // retain previous start state in case it was user-defined
		result.BlockID = blockID
		result.Chunks = ChunkListFixture(uint(chunks), block.ID())
		result.Chunks[0].StartState = startState // set start state to value before update
		result.PreviousResultID = previousResultID
	}
}

func ExecutionResultFixture(opts ...func(*flow.ExecutionResult)) *flow.ExecutionResult {
	blockID := IdentifierFixture()
	result := &flow.ExecutionResult{
		PreviousResultID: IdentifierFixture(),
		BlockID:          IdentifierFixture(),
		Chunks:           ChunkListFixture(2, blockID),
	}

	for _, apply := range opts {
		apply(result)
	}
	return result
}

func WithApproverID(approverID flow.Identifier) func(*flow.ResultApproval) {
	return func(ra *flow.ResultApproval) {
		ra.Body.ApproverID = approverID
	}
}

func WithAttestationBlock(block *flow.Block) func(*flow.ResultApproval) {
	return func(ra *flow.ResultApproval) {
		ra.Body.Attestation.BlockID = block.ID()
	}
}

func WithExecutionResultID(id flow.Identifier) func(*flow.ResultApproval) {
	return func(ra *flow.ResultApproval) {
		ra.Body.ExecutionResultID = id
	}
}

func WithBlockID(id flow.Identifier) func(*flow.ResultApproval) {
	return func(ra *flow.ResultApproval) {
		ra.Body.BlockID = id
	}
}

func WithChunk(chunkIdx uint64) func(*flow.ResultApproval) {
	return func(approval *flow.ResultApproval) {
		approval.Body.ChunkIndex = chunkIdx
	}
}

func WithServiveEvents(events ...flow.ServiceEvent) func(*flow.ExecutionResult) {
	return func(result *flow.ExecutionResult) {
		result.ServiceEvents = events
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

// WithAddress sets the network address of identity fixture.
func WithAddress(address string) func(*flow.Identity) {
	return func(identity *flow.Identity) {
		identity.Address = address
	}
}

// WithNetworkingKey sets the networking public key of identity fixture.
func WithNetworkingKey(key crypto.PublicKey) func(*flow.Identity) {
	return func(identity *flow.Identity) {
		identity.NetworkPubKey = key
	}
}

func RandomBytes(n int) []byte {
	b := make([]byte, n)
	read, err := crand.Read(b)
	if err != nil {
		panic("cannot read random bytes")
	}
	if read != n {
		panic(fmt.Errorf("cannot read enough random bytes (got %d of %d)", read, n))
	}
	return b
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
		identity.StakingPubKey = KeyFixture(crypto.BLSBLS12381).PublicKey()
		identity.NetworkPubKey = KeyFixture(crypto.ECDSAP256).PublicKey()
	}
}

// WithAllRoles can be used used to ensure an IdentityList fixtures contains
// all the roles required for a valid genesis block.
func WithAllRoles() func(*flow.Identity) {
	return WithAllRolesExcept()
}

// Same as above, but omitting a certain role for cases where we are manually
// setting up nodes or a particular role.
func WithAllRolesExcept(except ...flow.Role) func(*flow.Identity) {
	i := 0
	roles := flow.Roles()

	// remove omitted roles
	for _, omitRole := range except {
		for i, role := range roles {
			if role == omitRole {
				roles = append(roles[:i], roles[i+1:]...)
			}
		}
	}

	return func(id *flow.Identity) {
		id.Role = roles[i%len(roles)]
		i++
	}
}

// CompleteIdentitySet takes a number of identities and completes the missing roles.
func CompleteIdentitySet(identities ...*flow.Identity) flow.IdentityList {
	required := map[flow.Role]struct{}{
		flow.RoleCollection:   {},
		flow.RoleConsensus:    {},
		flow.RoleExecution:    {},
		flow.RoleVerification: {},
	}
	for _, identity := range identities {
		delete(required, identity.Role)
	}
	for role := range required {
		identities = append(identities, IdentityFixture(WithRole(role)))
	}
	return identities
}

// IdentityListFixture returns a list of node identity objects. The identities
// can be customized (ie. set their role) by passing in a function that modifies
// the input identities as required.
func IdentityListFixture(n int, opts ...func(*flow.Identity)) flow.IdentityList {
	identities := make(flow.IdentityList, n)

	for i := 0; i < n; i++ {
		identity := IdentityFixture()
		identity.Address = fmt.Sprintf("%x@flow.com:1234", identity.NodeID)
		for _, opt := range opts {
			opt(identity)
		}
		identities[i] = identity
	}

	return identities
}

func ChunkFixture(blockID flow.Identifier, collectionIndex uint) *flow.Chunk {
	return &flow.Chunk{
		ChunkBody: flow.ChunkBody{
			CollectionIndex:      collectionIndex,
			StartState:           StateCommitmentFixture(),
			EventCollection:      IdentifierFixture(),
			TotalComputationUsed: 4200,
			NumberOfTransactions: 42,
			BlockID:              blockID,
		},
		Index:    0,
		EndState: StateCommitmentFixture(),
	}
}

func ChunkListFixture(n uint, blockID flow.Identifier) flow.ChunkList {
	chunks := make([]*flow.Chunk, 0, n)
	for i := uint64(0); i < uint64(n); i++ {
		chunk := ChunkFixture(blockID, uint(i))
		chunk.Index = i
		chunks = append(chunks, chunk)
	}
	return chunks
}

func ChunkLocatorListFixture(n uint) chunks.LocatorList {
	locators := chunks.LocatorList{}
	resultID := IdentifierFixture()
	for i := uint64(0); i < uint64(n); i++ {
		locators = append(locators, ChunkLocatorFixture(resultID, i))
	}
	return locators
}

func ChunkLocatorFixture(resultID flow.Identifier, index uint64) *chunks.Locator {
	return &chunks.Locator{
		ResultID: resultID,
		Index:    index,
	}
}

func SignatureFixture() crypto.Signature {
	sig := make([]byte, 32)
	_, _ = crand.Read(sig)
	return sig
}

func CombinedSignatureFixture(n int) crypto.Signature {
	sigs := SignaturesFixture(n)
	combiner := signature.NewCombiner()
	sig, err := combiner.Join(sigs...)
	if err != nil {
		panic(err)
	}
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

func WithReferenceBlock(id flow.Identifier) func(tx *flow.TransactionBody) {
	return func(tx *flow.TransactionBody) {
		tx.ReferenceBlockID = id
	}
}

func TransactionDSLFixture(chain flow.Chain) dsl.Transaction {
	return dsl.Transaction{
		Import: dsl.Import{Address: sdk.Address(chain.ServiceAddress())},
		Content: dsl.Prepare{
			Content: dsl.Code(`
				pub fun main() {}
			`),
		},
	}
}

// VerifiableChunkDataFixture returns a complete verifiable chunk with an
// execution receipt referencing the block/collections.
func VerifiableChunkDataFixture(chunkIndex uint64) *verification.VerifiableChunkData {

	guarantees := make([]*flow.CollectionGuarantee, 0)

	var col flow.Collection

	for i := 0; i <= int(chunkIndex); i++ {
		col = CollectionFixture(1)
		guarantee := col.Guarantee()
		guarantees = append(guarantees, &guarantee)
	}

	payload := flow.Payload{
		Guarantees: guarantees,
		Seals:      nil,
	}
	header := BlockHeaderFixture()
	header.PayloadHash = payload.Hash()

	block := flow.Block{
		Header:  &header,
		Payload: &payload,
	}

	chunks := make([]*flow.Chunk, 0)

	var chunk flow.Chunk

	for i := 0; i <= int(chunkIndex); i++ {
		chunk = flow.Chunk{
			ChunkBody: flow.ChunkBody{
				CollectionIndex: uint(i),
				StartState:      StateCommitmentFixture(),
				BlockID:         block.ID(),
			},
			Index: uint64(i),
		}
		chunks = append(chunks, &chunk)
	}

	result := flow.ExecutionResult{
		BlockID: block.ID(),
		Chunks:  chunks,
	}

	// computes chunk end state
	index := chunk.Index
	var endState flow.StateCommitment
	if int(index) == len(result.Chunks)-1 {
		// last chunk in receipt takes final state commitment
		endState = StateCommitmentFixture()
	} else {
		// any chunk except last takes the subsequent chunk's start state
		endState = result.Chunks[index+1].StartState
	}

	return &verification.VerifiableChunkData{
		Chunk:         &chunk,
		Header:        block.Header,
		Result:        &result,
		Collection:    &col,
		ChunkDataPack: ChunkDataPackFixture(result.ID()),
		EndState:      endState,
	}
}

func ChunkDataPackFixture(identifier flow.Identifier) *flow.ChunkDataPack {

	//ids := utils.GetRandomRegisterIDs(1)
	//values := utils.GetRandomValues(1, 1, 32)

	return &flow.ChunkDataPack{
		ChunkID:    identifier,
		StartState: StateCommitmentFixture(),
		//RegisterTouches: []flow.RegisterTouch{{RegisterID: ids[0], Value: values[0], Proof: []byte{'p'}}},
		Proof:        []byte{'p'},
		CollectionID: IdentifierFixture(),
	}
}

// SeedFixture returns a random []byte with length n
func SeedFixture(n int) []byte {
	var seed = make([]byte, n)
	_, _ = crand.Read(seed)
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

func RangeFixture() flow.Range {
	return flow.Range{
		From: rand.Uint64(),
		To:   rand.Uint64(),
	}
}

func BatchFixture() flow.Batch {
	return flow.Batch{
		BlockIDs: IdentifierListFixture(10),
	}
}

func RangeListFixture(n int) []flow.Range {
	if n <= 0 {
		return nil
	}
	ranges := make([]flow.Range, n)
	for i := range ranges {
		ranges[i] = RangeFixture()
	}
	return ranges
}

func BatchListFixture(n int) []flow.Batch {
	if n <= 0 {
		return nil
	}
	batches := make([]flow.Batch, n)
	for i := range batches {
		batches[i] = BatchFixture()
	}
	return batches
}

func BootstrapExecutionResultFixture(block *flow.Block, commit flow.StateCommitment) *flow.ExecutionResult {
	result := &flow.ExecutionResult{
		BlockID:          block.ID(),
		PreviousResultID: flow.ZeroID,
		Chunks:           chunks.ChunkListFromCommit(commit),
	}
	return result
}

func KeyFixture(algo crypto.SigningAlgorithm) crypto.PrivateKey {
	key, err := crypto.GeneratePrivateKey(algo, SeedFixture(128))
	if err != nil {
		panic(err)
	}
	return key
}

func QuorumCertificateFixture(opts ...func(*flow.QuorumCertificate)) *flow.QuorumCertificate {
	qc := flow.QuorumCertificate{
		View:      uint64(rand.Uint32()),
		BlockID:   IdentifierFixture(),
		SignerIDs: IdentifierListFixture(3),
		SigData:   CombinedSignatureFixture(2),
	}
	for _, apply := range opts {
		apply(&qc)
	}
	return &qc
}

func QCWithBlockID(blockID flow.Identifier) func(*flow.QuorumCertificate) {
	return func(qc *flow.QuorumCertificate) {
		qc.BlockID = blockID
	}
}

func VoteFixture() *hotstuff.Vote {
	return &hotstuff.Vote{
		View:     uint64(rand.Uint32()),
		BlockID:  IdentifierFixture(),
		SignerID: IdentifierFixture(),
		SigData:  RandomBytes(128),
	}
}

func WithParticipants(participants flow.IdentityList) func(*flow.EpochSetup) {
	return func(setup *flow.EpochSetup) {
		setup.Participants = participants.Order(order.ByNodeIDAsc)
		setup.Assignments = ClusterAssignment(1, participants)
	}
}

func SetupWithCounter(counter uint64) func(*flow.EpochSetup) {
	return func(setup *flow.EpochSetup) {
		setup.Counter = counter
	}
}

func WithFinalView(view uint64) func(*flow.EpochSetup) {
	return func(setup *flow.EpochSetup) {
		setup.FinalView = view
	}
}

func WithFirstView(view uint64) func(*flow.EpochSetup) {
	return func(setup *flow.EpochSetup) {
		setup.FirstView = view
	}
}

func EpochSetupFixture(opts ...func(setup *flow.EpochSetup)) *flow.EpochSetup {
	participants := IdentityListFixture(5, WithAllRoles())
	setup := &flow.EpochSetup{
		Counter:      uint64(rand.Uint32()),
		FinalView:    uint64(rand.Uint32() + 1000),
		Participants: participants,
		RandomSource: SeedFixture(flow.EpochSetupRandomSourceLength),
	}
	for _, apply := range opts {
		apply(setup)
	}
	setup.Assignments = ClusterAssignment(1, setup.Participants)
	return setup
}

func EpochStatusFixture() *flow.EpochStatus {
	return &flow.EpochStatus{
		PreviousEpoch: flow.EventIDs{
			SetupID:  IdentifierFixture(),
			CommitID: IdentifierFixture(),
		},
		CurrentEpoch: flow.EventIDs{
			SetupID:  IdentifierFixture(),
			CommitID: IdentifierFixture(),
		},
		NextEpoch: flow.EventIDs{
			SetupID:  IdentifierFixture(),
			CommitID: IdentifierFixture(),
		},
	}
}

func IndexFixture() *flow.Index {
	return &flow.Index{
		CollectionIDs: IdentifierListFixture(5),
		SealIDs:       IdentifierListFixture(5),
		ReceiptIDs:    IdentifierListFixture(5),
	}
}

func WithDKGFromParticipants(participants flow.IdentityList) func(*flow.EpochCommit) {
	return func(commit *flow.EpochCommit) {
		lookup := make(map[flow.Identifier]flow.DKGParticipant)
		for i, node := range participants.Filter(filter.HasRole(flow.RoleConsensus)) {
			lookup[node.NodeID] = flow.DKGParticipant{
				Index:    uint(i),
				KeyShare: KeyFixture(crypto.BLSBLS12381).PublicKey(),
			}
		}
		commit.DKGParticipants = lookup
	}
}

func CommitWithCounter(counter uint64) func(*flow.EpochCommit) {
	return func(commit *flow.EpochCommit) {
		commit.Counter = counter
	}
}

func EpochCommitFixture(opts ...func(*flow.EpochCommit)) *flow.EpochCommit {
	commit := &flow.EpochCommit{
		Counter:         uint64(rand.Uint32()),
		ClusterQCs:      []*flow.QuorumCertificate{QuorumCertificateFixture()},
		DKGGroupKey:     KeyFixture(crypto.BLSBLS12381).PublicKey(),
		DKGParticipants: make(map[flow.Identifier]flow.DKGParticipant),
	}
	for _, apply := range opts {
		apply(commit)
	}
	return commit
}

// BootstrapFixture generates all the artifacts necessary to bootstrap the
// protocol state.
func BootstrapFixture(participants flow.IdentityList, opts ...func(*flow.Block)) (*flow.Block, *flow.ExecutionResult, *flow.Seal) {

	root := GenesisFixture(participants)
	for _, apply := range opts {
		apply(root)
	}

	counter := uint64(1)
	setup := EpochSetupFixture(
		WithParticipants(participants),
		SetupWithCounter(counter),
		WithFirstView(root.Header.View),
		WithFinalView(root.Header.View+1000),
	)
	commit := EpochCommitFixture(WithDKGFromParticipants(participants), CommitWithCounter(counter))

	result := BootstrapExecutionResultFixture(root, GenesisStateCommitment)
	result.ServiceEvents = []flow.ServiceEvent{setup.ServiceEvent(), commit.ServiceEvent()}

	seal := Seal.Fixture(Seal.WithResult(result))

	return root, result, seal
}

// RootSnapshotFixture returns a snapshot representing a root chain state, for
// example one as returned from BootstrapFixture.
func RootSnapshotFixture(participants flow.IdentityList, opts ...func(*flow.Block)) *inmem.Snapshot {
	block, result, seal := BootstrapFixture(participants, opts...)
	qc := QuorumCertificateFixture(QCWithBlockID(block.ID()))
	root, err := inmem.SnapshotFromBootstrapState(block, result, seal, qc)
	if err != nil {
		panic(err)
	}
	return root
}

// ChainFixture creates a list of blocks that forms a chain
func ChainFixture(nonGenesisCount int) ([]*flow.Block, *flow.ExecutionResult, *flow.Seal) {
	chain := make([]*flow.Block, 0, nonGenesisCount+1)

	participants := IdentityListFixture(5, WithAllRoles())
	genesis, result, seal := BootstrapFixture(participants)
	chain = append(chain, genesis)

	children := ChainFixtureFrom(nonGenesisCount, genesis.Header)
	chain = append(chain, children...)
	return chain, result, seal
}

// ChainFixtureFrom creates a chain of blocks starting from a given parent block,
// the total number of blocks in the chain is specified by the given count.
func ChainFixtureFrom(count int, parent *flow.Header) []*flow.Block {
	blocks := make([]*flow.Block, 0, count)

	for i := 0; i < count; i++ {
		block := BlockWithParentFixture(parent)
		blocks = append(blocks, &block)
		parent = block.Header
	}

	return blocks
}

func ReceiptChainFor(blocks []*flow.Block, result0 *flow.ExecutionResult) []*flow.ExecutionReceipt {
	receipts := make([]*flow.ExecutionReceipt, len(blocks))
	receipts[0] = ExecutionReceiptFixture(WithResult(result0))
	receipts[0].ExecutionResult.BlockID = blocks[0].ID()

	for i := 1; i < len(blocks); i++ {
		b := blocks[i]
		prevReceipt := receipts[i-1]
		receipt := ReceiptForBlockFixture(b)
		receipt.ExecutionResult.PreviousResultID = prevReceipt.ExecutionResult.ID()
		prevLastChunk := prevReceipt.ExecutionResult.Chunks[len(prevReceipt.ExecutionResult.Chunks)-1]
		receipt.ExecutionResult.Chunks[0].StartState = prevLastChunk.EndState
		receipts[i] = receipt
	}

	return receipts
}

// ReconnectBlocksAndReceipts re-computes each block's PayloadHash and ParentID
// so that all the blocks are connected.
// blocks' height have to be in strict increasing order.
func ReconnectBlocksAndReceipts(blocks []*flow.Block, receipts []*flow.ExecutionReceipt) {
	blocks[0].Header.PayloadHash = blocks[0].Payload.Hash()
	receipts[0].ExecutionResult.BlockID = blocks[0].ID()
	for _, chunk := range receipts[0].ExecutionResult.Chunks {
		chunk.BlockID = blocks[0].ID()
	}

	for i := 1; i < len(blocks); i++ {
		b := blocks[i]
		p := i - 1
		prev := blocks[p]
		if prev.Header.Height+1 != b.Header.Height {
			panic(fmt.Sprintf("height has gap when connecting blocks: expect %v, but got %v", prev.Header.Height+1, b.Header.Height))
		}
		b.Header.ParentID = prev.ID()
		b.Header.PayloadHash = b.Payload.Hash()
		receipts[i].ExecutionResult.BlockID = b.ID()
		prevReceipt := receipts[p]
		receipts[i].ExecutionResult.PreviousResultID = prevReceipt.ExecutionResult.ID()
		for _, c := range receipts[i].ExecutionResult.Chunks {
			c.BlockID = b.ID()
		}
	}
}
