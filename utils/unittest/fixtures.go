package unittest

import (
	"bytes"
	crand "crypto/rand"
	"fmt"
	"math/rand"
	"net"
	"testing"
	"time"

	"github.com/ipfs/go-cid"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	pubsub_pb "github.com/libp2p/go-libp2p-pubsub/pb"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/onflow/cadence"
	"github.com/stretchr/testify/require"

	sdk "github.com/onflow/flow-go-sdk"
	hotstuff "github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/crypto/hash"
	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/engine/access/rest/util"
	"github.com/onflow/flow-go/fvm/storage/snapshot"
	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/bitutils"
	"github.com/onflow/flow-go/ledger/common/testutils"
	"github.com/onflow/flow-go/model/bootstrap"
	"github.com/onflow/flow-go/model/chainsync"
	"github.com/onflow/flow-go/model/chunks"
	"github.com/onflow/flow-go/model/cluster"
	"github.com/onflow/flow-go/model/encoding"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/model/messages"
	"github.com/onflow/flow-go/model/verification"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/executiondatasync/execution_data"
	"github.com/onflow/flow-go/module/mempool/entity"
	"github.com/onflow/flow-go/module/signature"
	"github.com/onflow/flow-go/module/updatable_configs"
	"github.com/onflow/flow-go/network/channels"
	"github.com/onflow/flow-go/network/message"
	p2pconfig "github.com/onflow/flow-go/network/p2p/config"
	"github.com/onflow/flow-go/network/p2p/keyutils"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/state/protocol/inmem"
	"github.com/onflow/flow-go/utils/dsl"
)

const (
	DefaultSeedFixtureLength = 64
	DefaultAddress           = "localhost:0"
)

// returns a deterministic math/rand PRG that can be used for deterministic randomness in tests only.
// The PRG seed is logged in case the test iteration needs to be reproduced.
func GetPRG(t *testing.T) *rand.Rand {
	random := time.Now().UnixNano()
	t.Logf("rng seed is %d", random)
	rng := rand.New(rand.NewSource(random))
	return rng
}

func IPPort(port string) string {
	return net.JoinHostPort("localhost", port)
}

func AddressFixture() flow.Address {
	return flow.Testnet.Chain().ServiceAddress()
}

func RandomAddressFixture() flow.Address {
	return RandomAddressFixtureForChain(flow.Testnet)
}

func RandomAddressFixtureForChain(chainID flow.ChainID) flow.Address {
	// we use a 32-bit index - since the linear address generator uses 45 bits,
	// this won't error
	addr, err := chainID.Chain().AddressAtIndex(uint64(rand.Uint32()))
	if err != nil {
		panic(err)
	}
	return addr
}

// Uint64InRange returns a uint64 value drawn from the uniform random distribution [min,max].
func Uint64InRange(min, max uint64) uint64 {
	return min + uint64(rand.Intn(int(max)+1-int(min)))
}

func RandomSDKAddressFixture() sdk.Address {
	addr := RandomAddressFixture()
	var sdkAddr sdk.Address
	copy(sdkAddr[:], addr[:])
	return sdkAddr
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
	return AccountKeyFixture(crypto.KeyGenSeedMinLen, crypto.ECDSAP256, hash.SHA3_256)
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

// AccountFixture returns a randomly generated account.
func AccountFixture() (*flow.Account, error) {
	key, err := AccountKeyFixture(128, crypto.ECDSAP256, hash.SHA3_256)
	if err != nil {
		return nil, err
	}

	contracts := make(map[string][]byte, 2)
	contracts["contract1"] = []byte("contract1")
	contracts["contract2"] = []byte("contract2")

	return &flow.Account{
		Address:   RandomAddressFixture(),
		Balance:   100,
		Keys:      []flow.AccountPublicKey{key.PublicKey(1000)},
		Contracts: contracts,
	}, nil
}

func BlockFixture() flow.Block {
	header := BlockHeaderFixture()
	return *BlockWithParentFixture(header)
}

func FullBlockFixture() flow.Block {
	block := BlockFixture()
	payload := block.Payload
	payload.Seals = Seal.Fixtures(10)
	payload.Results = []*flow.ExecutionResult{
		ExecutionResultFixture(),
		ExecutionResultFixture(),
	}
	payload.Receipts = []*flow.ExecutionReceiptMeta{
		ExecutionReceiptFixture(WithResult(payload.Results[0])).Meta(),
		ExecutionReceiptFixture(WithResult(payload.Results[1])).Meta(),
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
	return messages.NewBlockProposal(block)
}

func ClusterProposalFromBlock(block *cluster.Block) *messages.ClusterBlockProposal {
	return messages.NewClusterBlockProposal(block)
}

func BlockchainFixture(length int) []*flow.Block {
	blocks := make([]*flow.Block, length)

	genesis := BlockFixture()
	blocks[0] = &genesis
	for i := 1; i < length; i++ {
		blocks[i] = BlockWithParentFixture(blocks[i-1].Header)
	}

	return blocks
}

// AsSlashable returns the input message T, wrapped as a flow.Slashable instance with a random origin ID.
func AsSlashable[T any](msg T) flow.Slashable[T] {
	slashable := flow.Slashable[T]{
		OriginID: IdentifierFixture(),
		Message:  msg,
	}
	return slashable
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
	for i := 0; i < 10; i++ {
		receipt := ExecutionReceiptFixture(
			WithResult(ExecutionResultFixture(WithServiceEvents(3))),
			WithSpocks(SignaturesFixture(3)),
		)
		payload.Receipts = flow.ExecutionReceiptMetaList{receipt.Meta()}
		payload.Results = flow.ExecutionResultList{&receipt.ExecutionResult}
	}
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
		for _, receipt := range receipts {
			payload.Receipts = append(payload.Receipts, receipt.Meta())
			payload.Results = append(payload.Results, &receipt.ExecutionResult)
		}
	}
}

// WithReceiptsAndNoResults will add receipt to payload only
func WithReceiptsAndNoResults(receipts ...*flow.ExecutionReceipt) func(*flow.Payload) {
	return func(payload *flow.Payload) {
		for _, receipt := range receipts {
			payload.Receipts = append(payload.Receipts, receipt.Meta())
		}
	}
}

// WithExecutionResults will add execution results to payload
func WithExecutionResults(results ...*flow.ExecutionResult) func(*flow.Payload) {
	return func(payload *flow.Payload) {
		for _, result := range results {
			payload.Results = append(payload.Results, result)
		}
	}
}

func BlockWithParentFixture(parent *flow.Header) *flow.Block {
	payload := PayloadFixture()
	header := BlockHeaderWithParentFixture(parent)
	header.PayloadHash = payload.Hash()
	return &flow.Block{
		Header:  header,
		Payload: &payload,
	}
}

func BlockWithGuaranteesFixture(guarantees []*flow.CollectionGuarantee) *flow.Block {
	payload := PayloadFixture(WithGuarantees(guarantees...))
	header := BlockHeaderFixture()
	header.PayloadHash = payload.Hash()
	return &flow.Block{
		Header:  header,
		Payload: &payload,
	}

}

func WithoutGuarantee(payload *flow.Payload) {
	payload.Guarantees = nil
}

func StateInteractionsFixture() *snapshot.ExecutionSnapshot {
	return &snapshot.ExecutionSnapshot{}
}

func BlockWithParentAndProposerFixture(
	t *testing.T,
	parent *flow.Header,
	proposer flow.Identifier,
) flow.Block {
	block := BlockWithParentFixture(parent)

	indices, err := signature.EncodeSignersToIndices(
		[]flow.Identifier{proposer}, []flow.Identifier{proposer})
	require.NoError(t, err)

	block.Header.ProposerID = proposer
	block.Header.ParentVoterIndices = indices
	if block.Header.LastViewTC != nil {
		block.Header.LastViewTC.SignerIndices = indices
		block.Header.LastViewTC.NewestQC.SignerIndices = indices
	}

	return *block
}

func BlockWithParentAndSeals(parent *flow.Header, seals []*flow.Header) *flow.Block {
	block := BlockWithParentFixture(parent)
	payload := flow.Payload{
		Guarantees: nil,
	}

	if len(seals) > 0 {
		payload.Seals = make([]*flow.Seal, len(seals))
		for i, seal := range seals {
			payload.Seals[i] = Seal.Fixture(
				Seal.WithBlockID(seal.ID()),
			)
		}
	}

	block.SetPayload(payload)
	return block
}

func GenesisFixture() *flow.Block {
	genesis := flow.Genesis(flow.Emulator)
	return genesis
}

func WithHeaderHeight(height uint64) func(header *flow.Header) {
	return func(header *flow.Header) {
		header.Height = height
	}
}

func HeaderWithView(view uint64) func(*flow.Header) {
	return func(header *flow.Header) {
		header.View = view
	}
}

func BlockHeaderFixture(opts ...func(header *flow.Header)) *flow.Header {
	height := 1 + uint64(rand.Uint32()) // avoiding edge case of height = 0 (genesis block)
	view := height + uint64(rand.Intn(1000))
	header := BlockHeaderWithParentFixture(&flow.Header{
		ChainID:  flow.Emulator,
		ParentID: IdentifierFixture(),
		Height:   height,
		View:     view,
	})

	for _, opt := range opts {
		opt(header)
	}

	return header
}

func BlockHeaderFixtureOnChain(
	chainID flow.ChainID,
	opts ...func(header *flow.Header),
) *flow.Header {
	height := 1 + uint64(rand.Uint32()) // avoiding edge case of height = 0 (genesis block)
	view := height + uint64(rand.Intn(1000))
	header := BlockHeaderWithParentFixture(&flow.Header{
		ChainID:  chainID,
		ParentID: IdentifierFixture(),
		Height:   height,
		View:     view,
	})

	for _, opt := range opts {
		opt(header)
	}

	return header
}

func BlockHeaderWithParentFixture(parent *flow.Header) *flow.Header {
	height := parent.Height + 1
	view := parent.View + 1 + uint64(rand.Intn(10)) // Intn returns [0, n)
	var lastViewTC *flow.TimeoutCertificate
	if view != parent.View+1 {
		newestQC := QuorumCertificateFixture(func(qc *flow.QuorumCertificate) {
			qc.View = parent.View
		})
		lastViewTC = &flow.TimeoutCertificate{
			View:          view - 1,
			NewestQCViews: []uint64{newestQC.View},
			NewestQC:      newestQC,
			SignerIndices: SignerIndicesFixture(4),
			SigData:       SignatureFixture(),
		}
	}
	return &flow.Header{
		ChainID:            parent.ChainID,
		ParentID:           parent.ID(),
		Height:             height,
		PayloadHash:        IdentifierFixture(),
		Timestamp:          time.Now().UTC(),
		View:               view,
		ParentView:         parent.View,
		ParentVoterIndices: SignerIndicesFixture(4),
		ParentVoterSigData: QCSigDataFixture(),
		ProposerID:         IdentifierFixture(),
		ProposerSigData:    SignatureFixture(),
		LastViewTC:         lastViewTC,
	}
}

func BlockHeaderWithParentWithSoRFixture(parent *flow.Header, source []byte) *flow.Header {
	height := parent.Height + 1
	view := parent.View + 1 + uint64(rand.Intn(10)) // Intn returns [0, n)
	var lastViewTC *flow.TimeoutCertificate
	if view != parent.View+1 {
		newestQC := QuorumCertificateFixture(func(qc *flow.QuorumCertificate) {
			qc.View = parent.View
		})
		lastViewTC = &flow.TimeoutCertificate{
			View:          view - 1,
			NewestQCViews: []uint64{newestQC.View},
			NewestQC:      newestQC,
			SignerIndices: SignerIndicesFixture(4),
			SigData:       SignatureFixture(),
		}
	}
	return &flow.Header{
		ChainID:            parent.ChainID,
		ParentID:           parent.ID(),
		Height:             height,
		PayloadHash:        IdentifierFixture(),
		Timestamp:          time.Now().UTC(),
		View:               view,
		ParentView:         parent.View,
		ParentVoterIndices: SignerIndicesFixture(4),
		ParentVoterSigData: QCSigDataWithSoRFixture(source),
		ProposerID:         IdentifierFixture(),
		ProposerSigData:    SignatureFixture(),
		LastViewTC:         lastViewTC,
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
		Header:  header,
		Payload: payload,
	}
}

func ClusterBlockChainFixture(n int) []cluster.Block {
	clusterBlocks := make([]cluster.Block, 0, n)

	parent := ClusterBlockFixture()

	for i := 0; i < n; i++ {
		block := ClusterBlockWithParent(&parent)
		clusterBlocks = append(clusterBlocks, block)
		parent = block
	}

	return clusterBlocks
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
	header.ParentView = parent.Header.View
	header.PayloadHash = payload.Hash()

	block := cluster.Block{
		Header:  header,
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
		CollectionID:  IdentifierFixture(),
		SignerIndices: RandomBytes(16),
		Signature:     SignatureFixture(),
	}
	for _, option := range options {
		option(guarantee)
	}
	return guarantee
}

func CollectionGuaranteesWithCollectionIDFixture(collections []*flow.Collection) []*flow.CollectionGuarantee {
	guarantees := make([]*flow.CollectionGuarantee, 0, len(collections))
	for i := 0; i < len(collections); i++ {
		guarantee := CollectionGuaranteeFixture(WithCollection(collections[i]))
		guarantees = append(guarantees, guarantee)
	}
	return guarantees
}

func CollectionGuaranteesFixture(
	n int,
	options ...func(*flow.CollectionGuarantee),
) []*flow.CollectionGuarantee {
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

func CollectionListFixture(n int, options ...func(*flow.Collection)) []*flow.Collection {
	collections := make([]*flow.Collection, n)
	for i := 0; i < n; i++ {
		collection := CollectionFixture(1, options...)
		collections[i] = &collection
	}

	return collections
}

func CollectionFixture(n int, options ...func(*flow.Collection)) flow.Collection {
	transactions := make([]*flow.TransactionBody, 0, n)

	for i := 0; i < n; i++ {
		tx := TransactionFixture()
		transactions = append(transactions, &tx.TransactionBody)
	}

	col := flow.Collection{Transactions: transactions}
	for _, opt := range options {
		opt(&col)
	}
	return col
}

func FixedReferenceBlockID() flow.Identifier {
	blockID := flow.Identifier{}
	blockID[0] = byte(1)
	return blockID
}

func CompleteCollectionFixture() *entity.CompleteCollection {
	txBody := TransactionBodyFixture()
	return &entity.CompleteCollection{
		Guarantee: &flow.CollectionGuarantee{
			CollectionID:     flow.Collection{Transactions: []*flow.TransactionBody{&txBody}}.ID(),
			Signature:        SignatureFixture(),
			ReferenceBlockID: FixedReferenceBlockID(),
			SignerIndices:    SignerIndicesFixture(1),
		},
		Transactions: []*flow.TransactionBody{&txBody},
	}
}

func CompleteCollectionFromTransactions(txs []*flow.TransactionBody) *entity.CompleteCollection {
	return &entity.CompleteCollection{
		Guarantee: &flow.CollectionGuarantee{
			CollectionID:     flow.Collection{Transactions: txs}.ID(),
			Signature:        SignatureFixture(),
			ReferenceBlockID: IdentifierFixture(),
			SignerIndices:    SignerIndicesFixture(3),
		},
		Transactions: txs,
	}
}

func ExecutableBlockFixture(
	collectionsSignerIDs [][]flow.Identifier,
	startState *flow.StateCommitment,
) *entity.ExecutableBlock {

	header := BlockHeaderFixture()
	return ExecutableBlockFixtureWithParent(collectionsSignerIDs, header, startState)
}

func ExecutableBlockFixtureWithParent(
	collectionsSignerIDs [][]flow.Identifier,
	parent *flow.Header,
	startState *flow.StateCommitment,
) *entity.ExecutableBlock {

	completeCollections := make(map[flow.Identifier]*entity.CompleteCollection, len(collectionsSignerIDs))
	block := BlockWithParentFixture(parent)
	block.Payload.Guarantees = nil

	for range collectionsSignerIDs {
		completeCollection := CompleteCollectionFixture()
		block.Payload.Guarantees = append(block.Payload.Guarantees, completeCollection.Guarantee)
		completeCollections[completeCollection.Guarantee.CollectionID] = completeCollection
	}

	block.Header.PayloadHash = block.Payload.Hash()

	executableBlock := &entity.ExecutableBlock{
		Block:               block,
		CompleteCollections: completeCollections,
		StartState:          startState,
	}
	return executableBlock
}

func ExecutableBlockFromTransactions(
	chain flow.ChainID,
	txss [][]*flow.TransactionBody,
) *entity.ExecutableBlock {

	completeCollections := make(map[flow.Identifier]*entity.CompleteCollection, len(txss))
	blockHeader := BlockHeaderFixtureOnChain(chain)
	block := *BlockWithParentFixture(blockHeader)
	block.Payload.Guarantees = nil

	for _, txs := range txss {
		cc := CompleteCollectionFromTransactions(txs)
		block.Payload.Guarantees = append(block.Payload.Guarantees, cc.Guarantee)
		completeCollections[cc.Guarantee.CollectionID] = cc
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

func WithSpocks(spocks []crypto.Signature) func(*flow.ExecutionReceipt) {
	return func(receipt *flow.ExecutionReceipt) {
		receipt.Spocks = spocks
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

func ReceiptForBlockExecutorFixture(
	block *flow.Block,
	executor flow.Identifier,
) *flow.ExecutionReceipt {
	result := ExecutionResultFixture(WithBlock(block))
	receipt := ExecutionReceiptFixture(WithResult(result), WithExecutorID(executor))
	return receipt
}

func ReceiptsForBlockFixture(
	block *flow.Block,
	ids []flow.Identifier,
) []*flow.ExecutionReceipt {
	result := ExecutionResultFixture(WithBlock(block))
	var ers []*flow.ExecutionReceipt
	for _, id := range ids {
		ers = append(ers, ExecutionReceiptFixture(WithResult(result), WithExecutorID(id)))
	}
	return ers
}

func WithPreviousResult(prevResult flow.ExecutionResult) func(*flow.ExecutionResult) {
	return func(result *flow.ExecutionResult) {
		result.PreviousResultID = prevResult.ID()
		finalState, err := prevResult.FinalStateCommitment()
		if err != nil {
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
		result.Chunks = ChunkListFixture(uint(chunks), blockID)
		result.Chunks[0].StartState = startState // set start state to value before update
		result.PreviousResultID = previousResultID
	}
}

func WithChunks(n uint) func(*flow.ExecutionResult) {
	return func(result *flow.ExecutionResult) {
		result.Chunks = ChunkListFixture(n, result.BlockID)
	}
}

func ExecutionResultListFixture(
	n int,
	opts ...func(*flow.ExecutionResult),
) []*flow.ExecutionResult {
	results := make([]*flow.ExecutionResult, 0, n)
	for i := 0; i < n; i++ {
		results = append(results, ExecutionResultFixture(opts...))
	}

	return results
}

func WithExecutionResultBlockID(blockID flow.Identifier) func(*flow.ExecutionResult) {
	return func(result *flow.ExecutionResult) {
		result.BlockID = blockID
		for _, chunk := range result.Chunks {
			chunk.BlockID = blockID
		}
	}
}

func WithFinalState(commit flow.StateCommitment) func(*flow.ExecutionResult) {
	return func(result *flow.ExecutionResult) {
		result.Chunks[len(result.Chunks)-1].EndState = commit
	}
}

func WithServiceEvents(n int) func(result *flow.ExecutionResult) {
	return func(result *flow.ExecutionResult) {
		result.ServiceEvents = ServiceEventsFixture(n)
	}
}

func WithExecutionDataID(id flow.Identifier) func(result *flow.ExecutionResult) {
	return func(result *flow.ExecutionResult) {
		result.ExecutionDataID = id
	}
}

func ServiceEventsFixture(n int) flow.ServiceEventList {
	sel := make(flow.ServiceEventList, n)

	for i := 0; i < n; i++ {
		switch i % 3 {
		case 0:
			sel[i] = EpochCommitFixture().ServiceEvent()
		case 1:
			sel[i] = EpochSetupFixture().ServiceEvent()
		case 2:
			sel[i] = VersionBeaconFixture().ServiceEvent()
		}
	}

	return sel
}

func ExecutionResultFixture(opts ...func(*flow.ExecutionResult)) *flow.ExecutionResult {
	executedBlockID := IdentifierFixture()
	result := &flow.ExecutionResult{
		PreviousResultID: IdentifierFixture(),
		BlockID:          executedBlockID,
		Chunks:           ChunkListFixture(2, executedBlockID),
		ExecutionDataID:  IdentifierFixture(),
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

func AttestationFixture() *flow.Attestation {
	return &flow.Attestation{
		BlockID:           IdentifierFixture(),
		ExecutionResultID: IdentifierFixture(),
		ChunkIndex:        uint64(0),
	}
}

func StateCommitmentFixture() flow.StateCommitment {
	var state flow.StateCommitment
	_, _ = crand.Read(state[:])
	return state
}

func StateCommitmentPointerFixture() *flow.StateCommitment {
	state := StateCommitmentFixture()
	return &state
}

func HashFixture(size int) hash.Hash {
	hash := make(hash.Hash, size)
	for i := 0; i < size; i++ {
		hash[i] = byte(i)
	}
	return hash
}

func IdentifierListFixture(n int) flow.IdentifierList {
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

func SignerIndicesFixture(n int) []byte {
	indices := bitutils.MakeBitVector(10)
	for i := 0; i < n; i++ {
		bitutils.SetBit(indices, 1)
	}
	return indices
}

func SignerIndicesByIndices(n int, indices []int) []byte {
	signers := bitutils.MakeBitVector(n)
	for _, i := range indices {
		bitutils.SetBit(signers, i)
	}
	return signers
}

// WithRole adds a role to an identity fixture.
func WithRole(role flow.Role) func(*flow.Identity) {
	return func(identity *flow.Identity) {
		identity.Role = role
	}
}

// WithWeight sets the weight on an identity fixture.
func WithWeight(weight uint64) func(*flow.Identity) {
	return func(identity *flow.Identity) {
		identity.Weight = weight
	}
}

func WithEjected(ejected bool) func(*flow.Identity) {
	return func(identity *flow.Identity) {
		identity.Ejected = ejected
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

func NodeConfigFixture(opts ...func(*flow.Identity)) bootstrap.NodeConfig {
	identity := IdentityFixture(opts...)
	return bootstrap.NodeConfig{
		Role:    identity.Role,
		Address: identity.Address,
		Weight:  identity.Weight,
	}
}

func NodeInfoFixture(opts ...func(*flow.Identity)) bootstrap.NodeInfo {
	opts = append(opts, WithKeys)
	return bootstrap.NodeInfoFromIdentity(IdentityFixture(opts...))
}

func NodeInfosFixture(n int, opts ...func(*flow.Identity)) []bootstrap.NodeInfo {
	opts = append(opts, WithKeys)
	il := IdentityListFixture(n, opts...)
	nodeInfos := make([]bootstrap.NodeInfo, 0, n)
	for _, identity := range il {
		nodeInfos = append(nodeInfos, bootstrap.NodeInfoFromIdentity(identity))
	}
	return nodeInfos
}

func PrivateNodeInfosFixture(n int, opts ...func(*flow.Identity)) []bootstrap.NodeInfo {
	il := IdentityListFixture(n, opts...)
	nodeInfos := make([]bootstrap.NodeInfo, 0, n)
	for _, identity := range il {
		nodeInfo := bootstrap.PrivateNodeInfoFromIdentity(identity, KeyFixture(crypto.ECDSAP256), KeyFixture(crypto.BLSBLS12381))
		nodeInfos = append(nodeInfos, nodeInfo)
	}
	return nodeInfos
}

// IdentityFixture returns a node identity.
func IdentityFixture(opts ...func(*flow.Identity)) *flow.Identity {
	nodeID := IdentifierFixture()
	stakingKey := StakingPrivKeyByIdentifier(nodeID)
	identity := flow.Identity{
		NodeID:        nodeID,
		Address:       fmt.Sprintf("address-%x", nodeID[0:7]),
		Role:          flow.RoleConsensus,
		Weight:        1000,
		StakingPubKey: stakingKey.PublicKey(),
	}
	for _, apply := range opts {
		apply(&identity)
	}
	return &identity
}

// IdentityWithNetworkingKeyFixture returns a node identity and networking private key
func IdentityWithNetworkingKeyFixture(opts ...func(*flow.Identity)) (
	*flow.Identity,
	crypto.PrivateKey,
) {
	networkKey := NetworkingPrivKeyFixture()
	opts = append(opts, WithNetworkingKey(networkKey.PublicKey()))
	id := IdentityFixture(opts...)
	return id, networkKey
}

func WithKeys(identity *flow.Identity) {
	staking := StakingPrivKeyFixture()
	networking := NetworkingPrivKeyFixture()
	identity.StakingPubKey = staking.PublicKey()
	identity.NetworkPubKey = networking.PublicKey()
}

// WithNodeID adds a node ID with the given first byte to an identity.
func WithNodeID(id flow.Identifier) func(*flow.Identity) {
	return func(identity *flow.Identity) {
		identity.NodeID = id
	}
}

// WithStakingPubKey adds a staking public key to the identity
func WithStakingPubKey(pubKey crypto.PublicKey) func(*flow.Identity) {
	return func(identity *flow.Identity) {
		identity.StakingPubKey = pubKey
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
	// don't add identities for roles that already exist
	for _, identity := range identities {
		delete(required, identity.Role)
	}
	// add identities for missing roles
	for role := range required {
		identities = append(identities, IdentityFixture(WithRole(role)))
	}
	return identities
}

// IdentityListFixture returns a list of node identity objects. The identities
// can be customized (ie. set their role) by passing in a function that modifies
// the input identities as required.
func IdentityListFixture(n int, opts ...func(*flow.Identity)) flow.IdentityList {
	identities := make(flow.IdentityList, 0, n)

	for i := 0; i < n; i++ {
		identity := IdentityFixture()
		identity.Address = fmt.Sprintf("%x@flow.com:1234", identity.NodeID)
		for _, opt := range opts {
			opt(identity)
		}
		identities = append(identities, identity)
	}

	return identities
}

func WithChunkStartState(startState flow.StateCommitment) func(chunk *flow.Chunk) {
	return func(chunk *flow.Chunk) {
		chunk.StartState = startState
	}
}

func ChunkFixture(
	blockID flow.Identifier,
	collectionIndex uint,
	opts ...func(*flow.Chunk),
) *flow.Chunk {
	chunk := &flow.Chunk{
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

	for _, opt := range opts {
		opt(chunk)
	}

	return chunk
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
		locator := ChunkLocatorFixture(resultID, i)
		locators = append(locators, locator)
	}
	return locators
}

func ChunkLocatorFixture(resultID flow.Identifier, index uint64) *chunks.Locator {
	return &chunks.Locator{
		ResultID: resultID,
		Index:    index,
	}
}

// ChunkStatusListToChunkLocatorFixture extracts chunk locators from a list of chunk statuses.
func ChunkStatusListToChunkLocatorFixture(statuses []*verification.ChunkStatus) chunks.LocatorMap {
	locators := chunks.LocatorMap{}
	for _, status := range statuses {
		locator := ChunkLocatorFixture(status.ExecutionResult.ID(), status.ChunkIndex)
		locators[locator.ID()] = locator
	}

	return locators
}

// ChunkStatusListFixture receives an execution result, samples `n` chunks out of it and
// creates a chunk status for them.
// It returns the list of sampled chunk statuses for the result.
func ChunkStatusListFixture(
	t *testing.T,
	blockHeight uint64,
	result *flow.ExecutionResult,
	n int,
) verification.ChunkStatusList {
	statuses := verification.ChunkStatusList{}

	// result should have enough chunk to sample
	require.GreaterOrEqual(t, len(result.Chunks), n)

	chunkList := make(flow.ChunkList, n)
	copy(chunkList, result.Chunks)
	rand.Shuffle(len(chunkList), func(i, j int) { chunkList[i], chunkList[j] = chunkList[j], chunkList[i] })

	for _, chunk := range chunkList[:n] {
		status := &verification.ChunkStatus{
			ChunkIndex:      chunk.Index,
			BlockHeight:     blockHeight,
			ExecutionResult: result,
		}
		statuses = append(statuses, status)
	}

	return statuses
}

func qcSignatureDataFixture() hotstuff.SignatureData {
	sigType := RandomBytes(5)
	for i := range sigType {
		sigType[i] = sigType[i] % 2
	}
	sigData := hotstuff.SignatureData{
		SigType:                      sigType,
		AggregatedStakingSig:         SignatureFixture(),
		AggregatedRandomBeaconSig:    SignatureFixture(),
		ReconstructedRandomBeaconSig: SignatureFixture(),
	}
	return sigData
}

func QCSigDataFixture() []byte {
	packer := hotstuff.SigDataPacker{}
	sigData := qcSignatureDataFixture()
	encoded, _ := packer.Encode(&sigData)
	return encoded
}

func QCSigDataWithSoRFixture(sor []byte) []byte {
	packer := hotstuff.SigDataPacker{}
	sigData := qcSignatureDataFixture()
	sigData.ReconstructedRandomBeaconSig = sor
	encoded, _ := packer.Encode(&sigData)
	return encoded
}

func SignatureFixture() crypto.Signature {
	sig := make([]byte, crypto.SignatureLenBLSBLS12381)
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

func RandomSourcesFixture(n int) [][]byte {
	var sigs [][]byte
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
		EnvelopeSignatures: []flow.TransactionSignature{TransactionSignatureFixture()},
	}

	for _, apply := range opts {
		apply(&tb)
	}

	return tb
}

func TransactionBodyListFixture(n int) []flow.TransactionBody {
	l := make([]flow.TransactionBody, n)
	for i := 0; i < n; i++ {
		l[i] = TransactionBodyFixture()
	}

	return l
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

// RegisterIDFixture returns a RegisterID with a fixed key and owner
func RegisterIDFixture() flow.RegisterID {
	return flow.NewRegisterID(RandomAddressFixture(), "key")
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
		Header:  header,
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
		ChunkDataPack: ChunkDataPackFixture(result.ID()),
		EndState:      endState,
	}
}

// ChunkDataResponseMsgFixture creates a chunk data response message with a single-transaction collection, and random chunk ID.
// Use options to customize the response.
func ChunkDataResponseMsgFixture(
	chunkID flow.Identifier,
	opts ...func(*messages.ChunkDataResponse),
) *messages.ChunkDataResponse {
	cdp := &messages.ChunkDataResponse{
		ChunkDataPack: *ChunkDataPackFixture(chunkID),
		Nonce:         rand.Uint64(),
	}

	for _, opt := range opts {
		opt(cdp)
	}

	return cdp
}

// WithApproximateSize sets the ChunkDataResponse to be approximately bytes in size.
func WithApproximateSize(bytes uint64) func(*messages.ChunkDataResponse) {
	return func(request *messages.ChunkDataResponse) {
		// 1 tx fixture is approximately 350 bytes
		txCount := bytes / 350
		collection := CollectionFixture(int(txCount) + 1)
		pack := ChunkDataPackFixture(request.ChunkDataPack.ChunkID, WithChunkDataPackCollection(&collection))
		request.ChunkDataPack = *pack
	}
}

// ChunkDataResponseMessageListFixture creates a list of chunk data response messages each with a single-transaction collection, and random chunk ID.
func ChunkDataResponseMessageListFixture(chunkIDs flow.IdentifierList) []*messages.ChunkDataResponse {
	lst := make([]*messages.ChunkDataResponse, 0, len(chunkIDs))
	for _, chunkID := range chunkIDs {
		lst = append(lst, ChunkDataResponseMsgFixture(chunkID))
	}
	return lst
}

// ChunkDataPackRequestListFixture creates and returns a list of chunk data pack requests fixtures.
func ChunkDataPackRequestListFixture(
	n int,
	opts ...func(*verification.ChunkDataPackRequest),
) verification.ChunkDataPackRequestList {
	lst := make([]*verification.ChunkDataPackRequest, 0, n)
	for i := 0; i < n; i++ {
		lst = append(lst, ChunkDataPackRequestFixture(opts...))
	}
	return lst
}

func WithHeight(height uint64) func(*verification.ChunkDataPackRequest) {
	return func(request *verification.ChunkDataPackRequest) {
		request.Height = height
	}
}

func WithHeightGreaterThan(height uint64) func(*verification.ChunkDataPackRequest) {
	return func(request *verification.ChunkDataPackRequest) {
		request.Height = height + 1
	}
}

func WithAgrees(list flow.IdentifierList) func(*verification.ChunkDataPackRequest) {
	return func(request *verification.ChunkDataPackRequest) {
		request.Agrees = list
	}
}

func WithDisagrees(list flow.IdentifierList) func(*verification.ChunkDataPackRequest) {
	return func(request *verification.ChunkDataPackRequest) {
		request.Disagrees = list
	}
}

func WithChunkID(chunkID flow.Identifier) func(*verification.ChunkDataPackRequest) {
	return func(request *verification.ChunkDataPackRequest) {
		request.ChunkID = chunkID
	}
}

// ChunkDataPackRequestFixture creates a chunk data request with some default values, i.e., one agree execution node, one disagree execution node,
// and height of zero.
// Use options to customize the request.
func ChunkDataPackRequestFixture(opts ...func(*verification.ChunkDataPackRequest)) *verification.
	ChunkDataPackRequest {

	req := &verification.ChunkDataPackRequest{
		Locator: chunks.Locator{
			ResultID: IdentifierFixture(),
			Index:    0,
		},
		ChunkDataPackRequestInfo: verification.ChunkDataPackRequestInfo{
			ChunkID:   IdentifierFixture(),
			Height:    0,
			Agrees:    IdentifierListFixture(1),
			Disagrees: IdentifierListFixture(1),
		},
	}

	for _, opt := range opts {
		opt(req)
	}

	// creates identity fixtures for target ids as union of agrees and disagrees
	// TODO: remove this inner fixture once we have filter for identifier list.
	targets := flow.IdentityList{}
	for _, id := range req.Agrees {
		targets = append(targets, IdentityFixture(WithNodeID(id), WithRole(flow.RoleExecution)))
	}
	for _, id := range req.Disagrees {
		targets = append(targets, IdentityFixture(WithNodeID(id), WithRole(flow.RoleExecution)))
	}

	req.Targets = targets

	return req
}

func WithChunkDataPackCollection(collection *flow.Collection) func(*flow.ChunkDataPack) {
	return func(cdp *flow.ChunkDataPack) {
		cdp.Collection = collection
	}
}

func WithStartState(startState flow.StateCommitment) func(*flow.ChunkDataPack) {
	return func(cdp *flow.ChunkDataPack) {
		cdp.StartState = startState
	}
}

func ChunkDataPackFixture(
	chunkID flow.Identifier,
	opts ...func(*flow.ChunkDataPack),
) *flow.ChunkDataPack {
	coll := CollectionFixture(1)
	cdp := &flow.ChunkDataPack{
		ChunkID:    chunkID,
		StartState: StateCommitmentFixture(),
		Proof:      []byte{'p'},
		Collection: &coll,
		ExecutionDataRoot: flow.BlockExecutionDataRoot{
			BlockID:               IdentifierFixture(),
			ChunkExecutionDataIDs: []cid.Cid{flow.IdToCid(IdentifierFixture())},
		},
	}

	for _, opt := range opts {
		opt(cdp)
	}

	return cdp
}

func ChunkDataPacksFixture(
	count int,
	opts ...func(*flow.ChunkDataPack),
) []*flow.ChunkDataPack {
	chunkDataPacks := make([]*flow.ChunkDataPack, count)
	for i := 0; i < count; i++ {
		chunkDataPacks[i] = ChunkDataPackFixture(IdentifierFixture())
	}

	return chunkDataPacks
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

// BlockEventsFixture returns a block events model populated with random events of length n.
func BlockEventsFixture(
	header *flow.Header,
	n int,
	types ...flow.EventType,
) flow.BlockEvents {
	return flow.BlockEvents{
		BlockID:        header.ID(),
		BlockHeight:    header.Height,
		BlockTimestamp: header.Timestamp,
		Events:         EventsFixture(n, types...),
	}
}

func EventsFixture(
	n int,
	types ...flow.EventType,
) []flow.Event {
	if len(types) == 0 {
		types = []flow.EventType{"A.0x1.Foo.Bar", "A.0x2.Zoo.Moo", "A.0x3.Goo.Hoo"}
	}

	events := make([]flow.Event, n)
	for i := 0; i < n; i++ {
		events[i] = EventFixture(types[i%len(types)], 0, uint32(i), IdentifierFixture(), 0)
	}

	return events
}

func EventTypeFixture(chainID flow.ChainID) flow.EventType {
	eventType := fmt.Sprintf("A.%s.TestContract.TestEvent1", RandomAddressFixtureForChain(chainID))
	return flow.EventType(eventType)
}

// EventFixture returns an event
func EventFixture(
	eType flow.EventType,
	transactionIndex uint32,
	eventIndex uint32,
	txID flow.Identifier,
	_ int,
) flow.Event {
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
	emulatorRootKey, err := crypto.DecodePrivateKey(crypto.ECDSAP256,
		[]byte("f87db87930770201010420ae2cc975dcbdd0ebc56f268b1d8a95834c2955970aea27042d35ec9f298b9e5aa00a06082a8648ce3d030107a1440342000417f5a527137785d2d773fee84b4c7ee40266a1dd1f36ddd46ecf25db6df6a499459629174de83256f2a44ebd4325b9def67d523b755a8926218c4efb7904f8ce0203"))
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

func RangeFixture() chainsync.Range {
	return chainsync.Range{
		From: rand.Uint64(),
		To:   rand.Uint64(),
	}
}

func BatchFixture() chainsync.Batch {
	return chainsync.Batch{
		BlockIDs: IdentifierListFixture(10),
	}
}

func RangeListFixture(n int) []chainsync.Range {
	if n <= 0 {
		return nil
	}
	ranges := make([]chainsync.Range, n)
	for i := range ranges {
		ranges[i] = RangeFixture()
	}
	return ranges
}

func BatchListFixture(n int) []chainsync.Batch {
	if n <= 0 {
		return nil
	}
	batches := make([]chainsync.Batch, n)
	for i := range batches {
		batches[i] = BatchFixture()
	}
	return batches
}

func BootstrapExecutionResultFixture(
	block *flow.Block,
	commit flow.StateCommitment,
) *flow.ExecutionResult {
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

func KeysFixture(n int, algo crypto.SigningAlgorithm) []crypto.PrivateKey {
	keys := make([]crypto.PrivateKey, 0, n)
	for i := 0; i < n; i++ {
		keys = append(keys, KeyFixture(algo))
	}
	return keys
}

func PublicKeysFixture(n int, algo crypto.SigningAlgorithm) []crypto.PublicKey {
	pks := make([]crypto.PublicKey, 0, n)
	sks := KeysFixture(n, algo)
	for _, sk := range sks {
		pks = append(pks, sk.PublicKey())
	}
	return pks
}

func QuorumCertificateWithSignerIDsFixture(opts ...func(*flow.QuorumCertificateWithSignerIDs)) *flow.QuorumCertificateWithSignerIDs {
	qc := flow.QuorumCertificateWithSignerIDs{
		View:      uint64(rand.Uint32()),
		BlockID:   IdentifierFixture(),
		SignerIDs: IdentifierListFixture(3),
		SigData:   QCSigDataFixture(),
	}
	for _, apply := range opts {
		apply(&qc)
	}
	return &qc
}

func QuorumCertificatesWithSignerIDsFixtures(
	n uint,
	opts ...func(*flow.QuorumCertificateWithSignerIDs),
) []*flow.QuorumCertificateWithSignerIDs {
	qcs := make([]*flow.QuorumCertificateWithSignerIDs, 0, n)
	for i := 0; i < int(n); i++ {
		qcs = append(qcs, QuorumCertificateWithSignerIDsFixture(opts...))
	}
	return qcs
}

func QuorumCertificatesFromAssignments(assignment flow.AssignmentList) []*flow.QuorumCertificateWithSignerIDs {
	qcs := make([]*flow.QuorumCertificateWithSignerIDs, 0, len(assignment))
	for _, nodes := range assignment {
		qc := QuorumCertificateWithSignerIDsFixture()
		qc.SignerIDs = nodes
		qcs = append(qcs, qc)
	}
	return qcs
}

func QuorumCertificateFixture(opts ...func(*flow.QuorumCertificate)) *flow.QuorumCertificate {
	qc := flow.QuorumCertificate{
		View:          uint64(rand.Uint32()),
		BlockID:       IdentifierFixture(),
		SignerIndices: SignerIndicesFixture(3),
		SigData:       QCSigDataFixture(),
	}
	for _, apply := range opts {
		apply(&qc)
	}
	return &qc
}

// CertifyBlock returns a quorum certificate for the given block header
func CertifyBlock(header *flow.Header) *flow.QuorumCertificate {
	qc := QuorumCertificateFixture(func(qc *flow.QuorumCertificate) {
		qc.View = header.View
		qc.BlockID = header.ID()
	})
	return qc
}

func QuorumCertificatesFixtures(
	n uint,
	opts ...func(*flow.QuorumCertificate),
) []*flow.QuorumCertificate {
	qcs := make([]*flow.QuorumCertificate, 0, n)
	for i := 0; i < int(n); i++ {
		qcs = append(qcs, QuorumCertificateFixture(opts...))
	}
	return qcs
}

func QCWithBlockID(blockID flow.Identifier) func(*flow.QuorumCertificate) {
	return func(qc *flow.QuorumCertificate) {
		qc.BlockID = blockID
	}
}

func QCWithSignerIndices(signerIndices []byte) func(*flow.QuorumCertificate) {
	return func(qc *flow.QuorumCertificate) {
		qc.SignerIndices = signerIndices
	}
}

func QCWithRootBlockID(blockID flow.Identifier) func(*flow.QuorumCertificate) {
	return func(qc *flow.QuorumCertificate) {
		qc.BlockID = blockID
		qc.View = 0
	}
}

func VoteFixture(opts ...func(vote *hotstuff.Vote)) *hotstuff.Vote {
	vote := &hotstuff.Vote{
		View:     uint64(rand.Uint32()),
		BlockID:  IdentifierFixture(),
		SignerID: IdentifierFixture(),
		SigData:  RandomBytes(128),
	}

	for _, opt := range opts {
		opt(vote)
	}

	return vote
}

func WithVoteSignerID(signerID flow.Identifier) func(*hotstuff.Vote) {
	return func(vote *hotstuff.Vote) {
		vote.SignerID = signerID
	}
}

func WithVoteView(view uint64) func(*hotstuff.Vote) {
	return func(vote *hotstuff.Vote) {
		vote.View = view
	}
}

func WithVoteBlockID(blockID flow.Identifier) func(*hotstuff.Vote) {
	return func(vote *hotstuff.Vote) {
		vote.BlockID = blockID
	}
}

func VoteForBlockFixture(
	block *hotstuff.Block,
	opts ...func(vote *hotstuff.Vote),
) *hotstuff.Vote {
	vote := VoteFixture(WithVoteView(block.View),
		WithVoteBlockID(block.BlockID))

	for _, opt := range opts {
		opt(vote)
	}

	return vote
}

func VoteWithStakingSig() func(*hotstuff.Vote) {
	return func(vote *hotstuff.Vote) {
		vote.SigData = append([]byte{byte(encoding.SigTypeStaking)}, vote.SigData...)
	}
}

func VoteWithBeaconSig() func(*hotstuff.Vote) {
	return func(vote *hotstuff.Vote) {
		vote.SigData = append([]byte{byte(encoding.SigTypeRandomBeacon)}, vote.SigData...)
	}
}

func WithParticipants(participants flow.IdentityList) func(*flow.EpochSetup) {
	return func(setup *flow.EpochSetup) {
		setup.Participants = participants.Sort(flow.Canonical)
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

// EpochSetupFixture creates a valid EpochSetup with default properties for
// testing. The default properties can be overwritten with optional parameter
// functions.
func EpochSetupFixture(opts ...func(setup *flow.EpochSetup)) *flow.EpochSetup {
	participants := IdentityListFixture(5, WithAllRoles())
	setup := &flow.EpochSetup{
		Counter:            uint64(rand.Uint32()),
		FirstView:          uint64(0),
		FinalView:          uint64(rand.Uint32() + 1000),
		Participants:       participants.Sort(flow.Canonical),
		RandomSource:       SeedFixture(flow.EpochSetupRandomSourceLength),
		DKGPhase1FinalView: 100,
		DKGPhase2FinalView: 200,
		DKGPhase3FinalView: 300,
	}
	for _, apply := range opts {
		apply(setup)
	}
	if setup.Assignments == nil {
		setup.Assignments = ClusterAssignment(1, setup.Participants)
	}
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
	count := len(participants.Filter(filter.IsValidDKGParticipant))
	return func(commit *flow.EpochCommit) {
		commit.DKGParticipantKeys = PublicKeysFixture(count, crypto.BLSBLS12381)
	}
}

func WithClusterQCsFromAssignments(assignments flow.AssignmentList) func(*flow.EpochCommit) {
	qcs := make([]*flow.QuorumCertificateWithSignerIDs, 0, len(assignments))
	for _, assignment := range assignments {
		qcWithSignerIndex := QuorumCertificateWithSignerIDsFixture()
		qcWithSignerIndex.SignerIDs = assignment
		qcs = append(qcs, qcWithSignerIndex)
	}
	return func(commit *flow.EpochCommit) {
		commit.ClusterQCs = flow.ClusterQCVoteDatasFromQCs(qcs)
	}
}

func DKGParticipantLookup(participants flow.IdentityList) map[flow.Identifier]flow.DKGParticipant {
	lookup := make(map[flow.Identifier]flow.DKGParticipant)
	for i, node := range participants.Filter(filter.HasRole(flow.RoleConsensus)) {
		lookup[node.NodeID] = flow.DKGParticipant{
			Index:    uint(i),
			KeyShare: KeyFixture(crypto.BLSBLS12381).PublicKey(),
		}
	}
	return lookup
}

func CommitWithCounter(counter uint64) func(*flow.EpochCommit) {
	return func(commit *flow.EpochCommit) {
		commit.Counter = counter
	}
}

func EpochCommitFixture(opts ...func(*flow.EpochCommit)) *flow.EpochCommit {
	commit := &flow.EpochCommit{
		Counter:            uint64(rand.Uint32()),
		ClusterQCs:         flow.ClusterQCVoteDatasFromQCs(QuorumCertificatesWithSignerIDsFixtures(1)),
		DKGGroupKey:        KeyFixture(crypto.BLSBLS12381).PublicKey(),
		DKGParticipantKeys: PublicKeysFixture(2, crypto.BLSBLS12381),
	}
	for _, apply := range opts {
		apply(commit)
	}
	return commit
}

func WithBoundaries(boundaries ...flow.VersionBoundary) func(*flow.VersionBeacon) {
	return func(b *flow.VersionBeacon) {
		b.VersionBoundaries = append(b.VersionBoundaries, boundaries...)
	}
}

func VersionBeaconFixture(options ...func(*flow.VersionBeacon)) *flow.VersionBeacon {

	versionTable := &flow.VersionBeacon{
		VersionBoundaries: []flow.VersionBoundary{},
		Sequence:          uint64(0),
	}
	opts := options

	if len(opts) == 0 {
		opts = []func(*flow.VersionBeacon){
			WithBoundaries(flow.VersionBoundary{
				Version:     "0.0.0",
				BlockHeight: 0,
			}),
		}
	}

	for _, apply := range opts {
		apply(versionTable)
	}

	return versionTable
}

// BootstrapFixture generates all the artifacts necessary to bootstrap the
// protocol state.
func BootstrapFixture(
	participants flow.IdentityList,
	opts ...func(*flow.Block),
) (*flow.Block, *flow.ExecutionResult, *flow.Seal) {
	return BootstrapFixtureWithChainID(participants, flow.Emulator, opts...)
}

func BootstrapFixtureWithChainID(
	participants flow.IdentityList,
	chainID flow.ChainID,
	opts ...func(*flow.Block),
) (*flow.Block, *flow.ExecutionResult, *flow.Seal) {

	root := flow.Genesis(chainID)
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
	commit := EpochCommitFixture(
		CommitWithCounter(counter),
		WithClusterQCsFromAssignments(setup.Assignments),
		WithDKGFromParticipants(participants),
	)

	stateCommit := GenesisStateCommitmentByChainID(chainID)
	result := BootstrapExecutionResultFixture(root, stateCommit)
	result.ServiceEvents = []flow.ServiceEvent{
		setup.ServiceEvent(),
		commit.ServiceEvent(),
	}

	seal := Seal.Fixture(Seal.WithResult(result))

	return root, result, seal
}

// RootSnapshotFixture returns a snapshot representing a root chain state, for
// example one as returned from BootstrapFixture.
func RootSnapshotFixture(
	participants flow.IdentityList,
	opts ...func(*flow.Block),
) *inmem.Snapshot {
	return RootSnapshotFixtureWithChainID(participants, flow.Emulator, opts...)
}

func RootSnapshotFixtureWithChainID(
	participants flow.IdentityList,
	chainID flow.ChainID,
	opts ...func(*flow.Block),
) *inmem.Snapshot {
	block, result, seal := BootstrapFixtureWithChainID(participants.Sort(flow.Canonical), chainID, opts...)
	qc := QuorumCertificateFixture(QCWithRootBlockID(block.ID()))
	root, err := inmem.SnapshotFromBootstrapState(block, result, seal, qc)
	if err != nil {
		panic(err)
	}
	return root
}

func SnapshotClusterByIndex(
	snapshot *inmem.Snapshot,
	clusterIndex uint,
) (protocol.Cluster, error) {
	epochs := snapshot.Epochs()
	epoch := epochs.Current()
	cluster, err := epoch.Cluster(clusterIndex)
	if err != nil {
		return nil, err
	}
	return cluster, nil
}

// ChainFixture creates a list of blocks that forms a chain
func ChainFixture(nonGenesisCount int) (
	[]*flow.Block,
	*flow.ExecutionResult,
	*flow.Seal,
) {
	chain := make([]*flow.Block, 0, nonGenesisCount+1)

	participants := IdentityListFixture(5, WithAllRoles())
	genesis, result, seal := BootstrapFixture(participants)
	chain = append(chain, genesis)

	children := ChainFixtureFrom(nonGenesisCount, genesis.Header)
	chain = append(chain, children...)
	return chain, result, seal
}

// ChainFixtureFrom creates a chain of blocks starting from a given parent block,
// the total number of blocks in the chain is specified by the given count
func ChainFixtureFrom(count int, parent *flow.Header) []*flow.Block {
	blocks := make([]*flow.Block, 0, count)

	for i := 0; i < count; i++ {
		block := BlockWithParentFixture(parent)
		blocks = append(blocks, block)
		parent = block.Header
	}

	return blocks
}

func ReceiptChainFor(
	blocks []*flow.Block,
	result0 *flow.ExecutionResult,
) []*flow.ExecutionReceipt {
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

	// after changing results we need to update IDs of results in receipt
	for _, block := range blocks {
		if len(block.Payload.Results) > 0 {
			for i := range block.Payload.Receipts {
				block.Payload.Receipts[i].ResultID = block.Payload.Results[i].ID()
			}
		}
	}
}

// DKGMessageFixture creates a single DKG message with random fields
func DKGMessageFixture() *messages.DKGMessage {
	return &messages.DKGMessage{
		Data:          RandomBytes(10),
		DKGInstanceID: fmt.Sprintf("test-dkg-instance-%d", uint64(rand.Int())),
	}
}

// DKGBroadcastMessageFixture creates a single DKG broadcast message with random fields
func DKGBroadcastMessageFixture() *messages.BroadcastDKGMessage {
	return &messages.BroadcastDKGMessage{
		DKGMessage:           *DKGMessageFixture(),
		CommitteeMemberIndex: uint64(rand.Int()),
		NodeID:               IdentifierFixture(),
		Signature:            SignatureFixture(),
	}
}

// PrivateKeyFixture returns a random private key with specified signature algorithm and seed length
func PrivateKeyFixture(algo crypto.SigningAlgorithm, seedLength int) crypto.PrivateKey {
	sk, err := crypto.GeneratePrivateKey(algo, SeedFixture(seedLength))
	if err != nil {
		panic(err)
	}
	return sk
}

// PrivateKeyFixtureByIdentifier returns a private key for a given node.
// given the same identifier, it will always return the same private key
func PrivateKeyFixtureByIdentifier(
	algo crypto.SigningAlgorithm,
	seedLength int,
	id flow.Identifier,
) crypto.PrivateKey {
	seed := append(id[:], id[:]...)
	sk, err := crypto.GeneratePrivateKey(algo, seed[:seedLength])
	if err != nil {
		panic(err)
	}
	return sk
}

func StakingPrivKeyByIdentifier(id flow.Identifier) crypto.PrivateKey {
	return PrivateKeyFixtureByIdentifier(crypto.BLSBLS12381, crypto.KeyGenSeedMinLen, id)
}

// NetworkingPrivKeyFixture returns random ECDSAP256 private key
func NetworkingPrivKeyFixture() crypto.PrivateKey {
	return PrivateKeyFixture(crypto.ECDSAP256, crypto.KeyGenSeedMinLen)
}

// StakingPrivKeyFixture returns a random BLS12381 private keyf
func StakingPrivKeyFixture() crypto.PrivateKey {
	return PrivateKeyFixture(crypto.BLSBLS12381, crypto.KeyGenSeedMinLen)
}

func NodeMachineAccountInfoFixture() bootstrap.NodeMachineAccountInfo {
	return bootstrap.NodeMachineAccountInfo{
		Address:           RandomAddressFixture().String(),
		EncodedPrivateKey: PrivateKeyFixture(crypto.ECDSAP256, DefaultSeedFixtureLength).Encode(),
		HashAlgorithm:     bootstrap.DefaultMachineAccountHashAlgo,
		SigningAlgorithm:  bootstrap.DefaultMachineAccountSignAlgo,
		KeyIndex:          bootstrap.DefaultMachineAccountKeyIndex,
	}
}

func MachineAccountFixture(t *testing.T) (
	bootstrap.NodeMachineAccountInfo,
	*sdk.Account,
) {
	info := NodeMachineAccountInfoFixture()

	bal, err := cadence.NewUFix64("0.5")
	require.NoError(t, err)

	acct := &sdk.Account{
		Address: sdk.HexToAddress(info.Address),
		Balance: uint64(bal),
		Keys: []*sdk.AccountKey{
			{
				Index:     int(info.KeyIndex),
				PublicKey: info.MustPrivateKey().PublicKey(),
				SigAlgo:   info.SigningAlgorithm,
				HashAlgo:  info.HashAlgorithm,
				Weight:    1000,
			},
		},
	}
	return info, acct
}

func TransactionResultsFixture(n int) []flow.TransactionResult {
	results := make([]flow.TransactionResult, 0, n)
	for i := 0; i < n; i++ {
		results = append(results, flow.TransactionResult{
			TransactionID:   IdentifierFixture(),
			ErrorMessage:    "whatever",
			ComputationUsed: uint64(rand.Uint32()),
		})
	}
	return results
}

func LightTransactionResultsFixture(n int) []flow.LightTransactionResult {
	results := make([]flow.LightTransactionResult, 0, n)
	for i := 0; i < n; i++ {
		results = append(results, flow.LightTransactionResult{
			TransactionID:   IdentifierFixture(),
			Failed:          i%2 == 0,
			ComputationUsed: Uint64InRange(1, 10_000),
		})
	}
	return results
}

func AllowAllPeerFilter() func(peer.ID) error {
	return func(_ peer.ID) error {
		return nil
	}
}

func NewSealingConfigs(val uint) module.SealingConfigsSetter {
	instance, err := updatable_configs.NewSealingConfigs(
		flow.DefaultRequiredApprovalsForSealConstruction,
		flow.DefaultRequiredApprovalsForSealValidation,
		flow.DefaultChunkAssignmentAlpha,
		flow.DefaultEmergencySealingActive,
	)
	if err != nil {
		panic(err)
	}
	err = instance.SetRequiredApprovalsForSealingConstruction(val)
	if err != nil {
		panic(err)
	}
	return instance
}

func PeerIDFromFlowID(identity *flow.Identity) (peer.ID, error) {
	networkKey := identity.NetworkPubKey
	peerPK, err := keyutils.LibP2PPublicKeyFromFlow(networkKey)
	if err != nil {
		return "", err
	}

	peerID, err := peer.IDFromPublicKey(peerPK)
	if err != nil {
		return "", err
	}

	return peerID, nil
}

func EngineMessageFixture() *engine.Message {
	return &engine.Message{
		OriginID: IdentifierFixture(),
		Payload:  RandomBytes(10),
	}
}

func EngineMessageFixtures(count int) []*engine.Message {
	messages := make([]*engine.Message, 0, count)
	for i := 0; i < count; i++ {
		messages = append(messages, EngineMessageFixture())
	}
	return messages
}

// GetFlowProtocolEventID returns the event ID for the event provided.
func GetFlowProtocolEventID(
	t *testing.T,
	channel channels.Channel,
	event interface{},
) flow.Identifier {
	payload, err := NetworkCodec().Encode(event)
	require.NoError(t, err)
	eventIDHash, err := message.EventId(channel, payload)
	require.NoError(t, err)
	return flow.HashToID(eventIDHash)
}

func WithBlockExecutionDataBlockID(blockID flow.Identifier) func(*execution_data.BlockExecutionData) {
	return func(bed *execution_data.BlockExecutionData) {
		bed.BlockID = blockID
	}
}

func WithChunkExecutionDatas(chunks ...*execution_data.ChunkExecutionData) func(*execution_data.BlockExecutionData) {
	return func(bed *execution_data.BlockExecutionData) {
		bed.ChunkExecutionDatas = chunks
	}
}

func BlockExecutionDataFixture(opts ...func(*execution_data.BlockExecutionData)) *execution_data.BlockExecutionData {
	bed := &execution_data.BlockExecutionData{
		BlockID:             IdentifierFixture(),
		ChunkExecutionDatas: []*execution_data.ChunkExecutionData{},
	}

	for _, opt := range opts {
		opt(bed)
	}

	return bed
}

func BlockExecutionDatEntityFixture(opts ...func(*execution_data.BlockExecutionData)) *execution_data.BlockExecutionDataEntity {
	execData := BlockExecutionDataFixture(opts...)
	return execution_data.NewBlockExecutionDataEntity(IdentifierFixture(), execData)
}

func BlockExecutionDatEntityListFixture(n int) []*execution_data.BlockExecutionDataEntity {
	l := make([]*execution_data.BlockExecutionDataEntity, n)
	for i := 0; i < n; i++ {
		l[i] = BlockExecutionDatEntityFixture()
	}

	return l
}

func WithChunkEvents(events flow.EventsList) func(*execution_data.ChunkExecutionData) {
	return func(conf *execution_data.ChunkExecutionData) {
		conf.Events = events
	}
}

func WithTrieUpdate(trieUpdate *ledger.TrieUpdate) func(*execution_data.ChunkExecutionData) {
	return func(conf *execution_data.ChunkExecutionData) {
		conf.TrieUpdate = trieUpdate
	}
}

func ChunkExecutionDataFixture(t *testing.T, minSize int, opts ...func(*execution_data.ChunkExecutionData)) *execution_data.ChunkExecutionData {
	collection := CollectionFixture(5)
	results := make([]flow.LightTransactionResult, len(collection.Transactions))
	for i, tx := range collection.Transactions {
		results[i] = flow.LightTransactionResult{
			TransactionID:   tx.ID(),
			Failed:          false,
			ComputationUsed: uint64(i * 100),
		}
	}

	ced := &execution_data.ChunkExecutionData{
		Collection:         &collection,
		Events:             nil,
		TrieUpdate:         testutils.TrieUpdateFixture(2, 1, 8),
		TransactionResults: results,
	}

	for _, opt := range opts {
		opt(ced)
	}

	if minSize <= 1 || ced.TrieUpdate == nil {
		return ced
	}

	size := 1
	for {
		buf := &bytes.Buffer{}
		require.NoError(t, execution_data.DefaultSerializer.Serialize(buf, ced))
		if buf.Len() >= minSize {
			return ced
		}

		v := make([]byte, size)
		_, err := crand.Read(v)
		require.NoError(t, err)

		k, err := ced.TrieUpdate.Payloads[0].Key()
		require.NoError(t, err)

		ced.TrieUpdate.Payloads[0] = ledger.NewPayload(k, v)
		size *= 2
	}
}

func CreateSendTxHttpPayload(tx flow.TransactionBody) map[string]interface{} {
	tx.Arguments = [][]uint8{} // fix how fixture creates nil values
	auth := make([]string, len(tx.Authorizers))
	for i, a := range tx.Authorizers {
		auth[i] = a.String()
	}

	return map[string]interface{}{
		"script":             util.ToBase64(tx.Script),
		"arguments":          tx.Arguments,
		"reference_block_id": tx.ReferenceBlockID.String(),
		"gas_limit":          fmt.Sprintf("%d", tx.GasLimit),
		"payer":              tx.Payer.String(),
		"proposal_key": map[string]interface{}{
			"address":         tx.ProposalKey.Address.String(),
			"key_index":       fmt.Sprintf("%d", tx.ProposalKey.KeyIndex),
			"sequence_number": fmt.Sprintf("%d", tx.ProposalKey.SequenceNumber),
		},
		"authorizers": auth,
		"payload_signatures": []map[string]interface{}{{
			"address":   tx.PayloadSignatures[0].Address.String(),
			"key_index": fmt.Sprintf("%d", tx.PayloadSignatures[0].KeyIndex),
			"signature": util.ToBase64(tx.PayloadSignatures[0].Signature),
		}},
		"envelope_signatures": []map[string]interface{}{{
			"address":   tx.EnvelopeSignatures[0].Address.String(),
			"key_index": fmt.Sprintf("%d", tx.EnvelopeSignatures[0].KeyIndex),
			"signature": util.ToBase64(tx.EnvelopeSignatures[0].Signature),
		}},
	}
}

// P2PRPCGraftFixtures returns n number of control message rpc Graft fixtures.
func P2PRPCGraftFixtures(topics ...string) []*pubsub_pb.ControlGraft {
	n := len(topics)
	grafts := make([]*pubsub_pb.ControlGraft, n)
	for i := 0; i < n; i++ {
		grafts[i] = P2PRPCGraftFixture(&topics[i])
	}
	return grafts
}

// P2PRPCGraftFixture returns a control message rpc Graft fixture.
func P2PRPCGraftFixture(topic *string) *pubsub_pb.ControlGraft {
	return &pubsub_pb.ControlGraft{
		TopicID: topic,
	}
}

// P2PRPCPruneFixtures returns n number of control message rpc Prune fixtures.
func P2PRPCPruneFixtures(topics ...string) []*pubsub_pb.ControlPrune {
	n := len(topics)
	prunes := make([]*pubsub_pb.ControlPrune, n)
	for i := 0; i < n; i++ {
		prunes[i] = P2PRPCPruneFixture(&topics[i])
	}
	return prunes
}

// P2PRPCPruneFixture returns a control message rpc Prune fixture.
func P2PRPCPruneFixture(topic *string) *pubsub_pb.ControlPrune {
	return &pubsub_pb.ControlPrune{
		TopicID: topic,
	}
}

// P2PRPCIHaveFixtures returns n number of control message where n = len(topics) rpc iHave fixtures with m number of message ids each.
func P2PRPCIHaveFixtures(m int, topics ...string) []*pubsub_pb.ControlIHave {
	n := len(topics)
	ihaves := make([]*pubsub_pb.ControlIHave, n)
	for i := 0; i < n; i++ {
		ihaves[i] = P2PRPCIHaveFixture(&topics[i], IdentifierListFixture(m).Strings()...)
	}
	return ihaves
}

// P2PRPCIHaveFixture returns a control message rpc iHave fixture.
func P2PRPCIHaveFixture(topic *string, messageIds ...string) *pubsub_pb.ControlIHave {
	return &pubsub_pb.ControlIHave{
		TopicID:    topic,
		MessageIDs: messageIds,
	}
}

// P2PRPCIWantFixtures returns n number of control message rpc iWant fixtures with m number of message ids each.
func P2PRPCIWantFixtures(n, m int) []*pubsub_pb.ControlIWant {
	iwants := make([]*pubsub_pb.ControlIWant, n)
	for i := 0; i < n; i++ {
		iwants[i] = P2PRPCIWantFixture(IdentifierListFixture(m).Strings()...)
	}
	return iwants
}

// P2PRPCIWantFixture returns a control message rpc iWant fixture.
func P2PRPCIWantFixture(messageIds ...string) *pubsub_pb.ControlIWant {
	return &pubsub_pb.ControlIWant{
		MessageIDs: messageIds,
	}
}

type RPCFixtureOpt func(rpc *pubsub.RPC)

// WithGrafts sets the grafts on the rpc control message.
func WithGrafts(grafts ...*pubsub_pb.ControlGraft) RPCFixtureOpt {
	return func(rpc *pubsub.RPC) {
		rpc.Control.Graft = grafts
	}
}

// WithPrunes sets the prunes on the rpc control message.
func WithPrunes(prunes ...*pubsub_pb.ControlPrune) RPCFixtureOpt {
	return func(rpc *pubsub.RPC) {
		rpc.Control.Prune = prunes
	}
}

// WithIHaves sets the iHaves on the rpc control message.
func WithIHaves(iHaves ...*pubsub_pb.ControlIHave) RPCFixtureOpt {
	return func(rpc *pubsub.RPC) {
		rpc.Control.Ihave = iHaves
	}
}

// WithIWants sets the iWants on the rpc control message.
func WithIWants(iWants ...*pubsub_pb.ControlIWant) RPCFixtureOpt {
	return func(rpc *pubsub.RPC) {
		rpc.Control.Iwant = iWants
	}
}

func WithPubsubMessages(msgs ...*pubsub_pb.Message) RPCFixtureOpt {
	return func(rpc *pubsub.RPC) {
		rpc.Publish = msgs
	}
}

// P2PRPCFixture returns a pubsub RPC fixture. Currently, this fixture only sets the ControlMessage field.
func P2PRPCFixture(opts ...RPCFixtureOpt) *pubsub.RPC {
	rpc := &pubsub.RPC{
		RPC: pubsub_pb.RPC{
			Control: &pubsub_pb.ControlMessage{},
		},
	}

	for _, opt := range opts {
		opt(rpc)
	}

	return rpc
}

func WithFrom(pid peer.ID) func(*pubsub_pb.Message) {
	return func(msg *pubsub_pb.Message) {
		msg.From = []byte(pid)
	}
}

// GossipSubMessageFixture returns a gossip sub message fixture for the specified topic.
func GossipSubMessageFixture(s string, opts ...func(*pubsub_pb.Message)) *pubsub_pb.Message {
	pb := &pubsub_pb.Message{
		From:      RandomBytes(32),
		Data:      RandomBytes(32),
		Seqno:     RandomBytes(10),
		Topic:     &s,
		Signature: RandomBytes(100),
		Key:       RandomBytes(32),
	}

	for _, opt := range opts {
		opt(pb)
	}

	return pb
}

// GossipSubMessageFixtures returns a list of gossipsub message fixtures.
func GossipSubMessageFixtures(n int, topic string, opts ...func(*pubsub_pb.Message)) []*pubsub_pb.Message {
	msgs := make([]*pubsub_pb.Message, n)
	for i := 0; i < n; i++ {
		msgs[i] = GossipSubMessageFixture(topic, opts...)
	}
	return msgs
}

// LibP2PResourceLimitOverrideFixture returns a random resource limit override for testing.
// The values are not guaranteed to be valid between 0 and 1000.
// Returns:
//   - p2pconf.ResourceManagerOverrideLimit: a random resource limit override.
func LibP2PResourceLimitOverrideFixture() p2pconfig.ResourceManagerOverrideLimit {
	return p2pconfig.ResourceManagerOverrideLimit{
		StreamsInbound:      rand.Intn(1000),
		StreamsOutbound:     rand.Intn(1000),
		ConnectionsInbound:  rand.Intn(1000),
		ConnectionsOutbound: rand.Intn(1000),
		FD:                  rand.Intn(1000),
		Memory:              rand.Intn(1000),
	}
}

func RegisterEntryFixture() flow.RegisterEntry {
	val := make([]byte, 4)
	_, _ = crand.Read(val)
	return flow.RegisterEntry{
		Key: flow.RegisterID{
			Owner: "owner",
			Key:   "key1",
		},
		Value: val,
	}
}

func MakeOwnerReg(key string, value string) flow.RegisterEntry {
	return flow.RegisterEntry{
		Key: flow.RegisterID{
			Owner: "owner",
			Key:   key,
		},
		Value: []byte(value),
	}
}
