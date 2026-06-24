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
	"github.com/onflow/crypto"
	"github.com/onflow/crypto/hash"
	"github.com/stretchr/testify/require"

	sdk "github.com/onflow/flow-go-sdk"

	hotstuff "github.com/onflow/flow-go/consensus/hotstuff/model"
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
	"github.com/onflow/flow-go/model/flow/mapfunc"
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
	"github.com/onflow/flow-go/state/protocol/protocol_state"
	"github.com/onflow/flow-go/state/protocol/protocol_state/kvstore"
	"github.com/onflow/flow-go/utils/dsl"
)

const (
	DefaultAddress = "localhost:0"
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

func SignatureFixtureForTransactions() crypto.Signature {
	sigLen := crypto.SignatureLenECDSAP256
	sig := SeedFixture(sigLen)

	// make sure the ECDSA signature passes the format check
	sig[sigLen/2] = 0
	sig[0] = 0
	sig[sigLen/2-1] |= 1
	sig[sigLen-1] |= 1
	return sig
}

func TransactionSignatureFixture() flow.TransactionSignature {
	s := flow.TransactionSignature{
		Address:       AddressFixture(),
		SignerIndex:   0,
		Signature:     SignatureFixtureForTransactions(),
		KeyIndex:      1,
		ExtensionData: []byte{},
	}

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

func ChainBlockFixtureWithRoot(root *flow.Header, n int) []*flow.Block {
	bs := make([]*flow.Block, 0, n)
	parent := root
	for i := 0; i < n; i++ {
		b := BlockWithParentFixture(parent)
		bs = append(bs, b)
		parent = b.ToHeader()
	}
	return bs
}

func RechainBlocks(blocks []*flow.Block) {
	if len(blocks) == 0 {
		return
	}

	parent := blocks[0]

	for _, block := range blocks[1:] {
		block.ParentID = parent.ID()
		parent = block
	}
}

func FullBlockFixture() *flow.Block {
	b := BlockFixture()
	return &flow.Block{
		HeaderBody: b.HeaderBody,
		Payload:    PayloadFixture(WithAllTheFixins),
	}
}

func BlockFixtures(number int) []*flow.Block {
	blocks := make([]*flow.Block, 0, number)
	for range number {
		block := BlockFixture()
		blocks = append(blocks, block)
	}
	return blocks
}

func ProposalFixtures(number int) []*flow.Proposal {
	proposals := make([]*flow.Proposal, 0, number)
	for range number {
		proposal := ProposalFixture()
		proposals = append(proposals, proposal)
	}
	return proposals
}

func ProposalFixture() *flow.Proposal {
	return ProposalFromBlock(BlockFixture())
}

func BlockResponseFixture(count int) *flow.BlockResponse {
	blocks := make([]flow.Proposal, count)
	for i := 0; i < count; i++ {
		blocks[i] = *ProposalFixture()
	}
	return &flow.BlockResponse{
		Nonce:  rand.Uint64(),
		Blocks: blocks,
	}
}

func ClusterProposalFixture() *cluster.Proposal {
	return ClusterProposalFromBlock(ClusterBlockFixture())
}

func ClusterBlockResponseFixture(count int) *cluster.BlockResponse {
	blocks := make([]cluster.Proposal, count)
	for i := 0; i < count; i++ {
		blocks[i] = *ClusterProposalFixture()
	}
	return &cluster.BlockResponse{
		Nonce:  rand.Uint64(),
		Blocks: blocks,
	}
}

func ProposalHeaderFromHeader(header *flow.Header) *flow.ProposalHeader {
	return &flow.ProposalHeader{
		Header:          header,
		ProposerSigData: SignatureFixture(),
	}
}

func ProposalFromBlock(block *flow.Block) *flow.Proposal {
	return &flow.Proposal{
		Block:           *block,
		ProposerSigData: SignatureFixture(),
	}
}

func ClusterProposalFromBlock(block *cluster.Block) *cluster.Proposal {
	return &cluster.Proposal{
		Block:           *block,
		ProposerSigData: SignatureFixture(),
	}
}

func BlockchainFixture(length int) []*flow.Block {
	blocks := make([]*flow.Block, length)

	genesis := BlockFixture()
	blocks[0] = genesis
	for i := 1; i < length; i++ {
		blocks[i] = BlockWithParentFixture(blocks[i-1].ToHeader())
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

// ReceiptAndSealForBlock returns a receipt with service events and a seal for them for a given block.
func ReceiptAndSealForBlock(block *flow.Block, serviceEvents ...flow.ServiceEvent) (*flow.ExecutionReceipt, *flow.Seal) {
	receipt := ReceiptForBlockFixture(block)
	receipt.ExecutionResult.ServiceEvents = serviceEvents
	seal := Seal.Fixture(Seal.WithBlock(block.ToHeader()), Seal.WithResult(&receipt.ExecutionResult))
	return receipt, seal
}

func PayloadFixture(options ...func(*flow.Payload)) flow.Payload {
	payload := flow.Payload{
		ProtocolStateID: IdentifierFixture(),
	}
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
		payload.Receipts = append(payload.Receipts, receipt.Stub())
		payload.Results = append(payload.Results, &receipt.ExecutionResult)
	}
	payload.ProtocolStateID = IdentifierFixture()
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
			payload.Receipts = append(payload.Receipts, receipt.Stub())
			payload.Results = append(payload.Results, &receipt.ExecutionResult)
		}
	}
}

func WithProtocolStateID(stateID flow.Identifier) func(payload *flow.Payload) {
	return func(payload *flow.Payload) {
		payload.ProtocolStateID = stateID
	}
}

// WithReceiptsAndNoResults will add receipt to payload only
func WithReceiptsAndNoResults(receipts ...*flow.ExecutionReceipt) func(*flow.Payload) {
	return func(payload *flow.Payload) {
		for _, receipt := range receipts {
			payload.Receipts = append(payload.Receipts, receipt.Stub())
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
	return BlockWithParentAndPayload(parent, PayloadFixture())
}

// BlockWithParentAndPayload creates a new block that is valid
// with respect to the given parent block and with given payload.
func BlockWithParentAndPayload(parent *flow.Header, payload flow.Payload) *flow.Block {
	return &flow.Block{
		HeaderBody: HeaderBodyWithParentFixture(parent),
		Payload:    payload,
	}
}

// BlockWithParentAndUniqueView creates a child block of the given parent.
// We provide a set of views that are _not_ allowed to be used for the new block. A typical usage
// scenario is to create blocks of different forks, without accidentally creating two blocks with
// the same view.
// CAUTION:
//   - modifies the set `forbiddenViews` by adding the view of the newly created block.
//   - To generate the child's view, we randomly select a small increment and add it to the
//     parent's view. If the set of views covers all possible increments, this function will panic
func BlockWithParentAndUniqueView(parent *flow.Header, forbiddenViews map[uint64]struct{}) *flow.Block {
	var block *flow.Block
	counter := 0
	for {
		block = BlockWithParentFixture(parent)
		if _, hasForbiddenView := forbiddenViews[block.View]; !hasForbiddenView {
			break
		}
		counter += 1
		if counter > 20 {
			panic(fmt.Sprintf("BlockWithParentAndUniqueView failed to generate child despite %d attempts", counter))
		}
	}
	// block has a view that is not forbidden:
	forbiddenViews[block.View] = struct{}{} // add the block's view to `forbiddenViews` to prevent future re-usage
	return block
}

// BlockWithParentAndPayloadAndUniqueView creates a child block of the given parent, where the block
// to be constructed will contain the iven payload.
// We provide a set of views that are _not_ allowed to be used for the new block. A typical usage
// scenario is to create blocks of different forks, without accidentally creating two blocks with
// the same view.
// CAUTION:
//   - modifies the set `forbiddenViews` by adding the view of the newly created block.
//   - To generate the child's view, we randomly select a small increment and add it to the
//     parent's view. If the set of views covers all possible increments, this function will panic
func BlockWithParentAndPayloadAndUniqueView(parent *flow.Header, payload flow.Payload, forbiddenViews map[uint64]struct{}) *flow.Block {
	var block *flow.Block
	counter := 0
	for {
		block = BlockWithParentAndPayload(parent, payload)
		if _, hasForbiddenView := forbiddenViews[block.View]; !hasForbiddenView {
			break
		}
		counter += 1
		if counter > 20 {
			panic(fmt.Sprintf("BlockWithParentAndUniqueView failed to generate child despite %d attempts", counter))
		}
	}
	// block has a view that is not forbidden:
	forbiddenViews[block.View] = struct{}{} // add the block's view to `forbiddenViews` to prevent future re-usage
	return block
}

func BlockWithParentProtocolState(parent *flow.Block) *flow.Block {
	return &flow.Block{
		HeaderBody: HeaderBodyWithParentFixture(parent.ToHeader()),
		Payload:    PayloadFixture(WithProtocolStateID(parent.Payload.ProtocolStateID)),
	}
}

// BlockWithParentProtocolStateAndUniqueView creates a child block of the given parent, such that
// the child's protocol state is the same as the parent's.
// We provide a set of views that are _not_ allowed to be used for the new block. A typical usage
// scenario is to create blocks of different forks, without accidentally creating two blocks with
// the same view.
// CAUTION:
//   - modifies the set `forbiddenViews` by adding the view of the newly created block.
//   - To generate the child's view, we randomly select a small increment and add it to the
//     parent's view. If the set of views covers all possible increments, this function will panic
func BlockWithParentProtocolStateAndUniqueView(parent *flow.Block, forbiddenViews map[uint64]struct{}) *flow.Block {
	var block *flow.Block
	counter := 0
	for {
		block = BlockWithParentProtocolState(parent)
		if _, hasForbiddenView := forbiddenViews[block.View]; !hasForbiddenView {
			break
		}
		counter += 1
		if counter > 20 {
			panic(fmt.Sprintf("BlockWithParentProtocolStateAndUniqueView failed to generate child despite %d attempts", counter))
		}
	}
	// block has a view that is not forbidden:
	forbiddenViews[block.View] = struct{}{} // add the block's view to `forbiddenViews` to prevent future re-usage
	return block
}

func BlockWithGuaranteesFixture(guarantees []*flow.CollectionGuarantee) *flow.Block {
	return &flow.Block{
		HeaderBody: HeaderBodyFixture(),
		Payload:    PayloadFixture(WithGuarantees(guarantees...)),
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
) *flow.Block {
	block := BlockWithParentFixture(parent)

	indices, err := signature.EncodeSignersToIndices(
		[]flow.Identifier{proposer}, []flow.Identifier{proposer})
	require.NoError(t, err)

	block.ProposerID = proposer
	block.ParentVoterIndices = indices
	if block.LastViewTC != nil {
		block.LastViewTC.SignerIndices = indices
		block.LastViewTC.NewestQC.SignerIndices = indices
	}

	return block
}

func BlockWithParentAndSeals(parent *flow.Header, seals []*flow.Header) *flow.Block {
	payload := flow.Payload{}

	if len(seals) > 0 {
		payload.Seals = make([]*flow.Seal, len(seals))
		for i, seal := range seals {
			payload.Seals[i] = Seal.Fixture(
				Seal.WithBlockID(seal.ID()),
			)
		}
	}
	return BlockWithParentAndPayload(parent, payload)
}

func WithHeaderHeight(height uint64) func(*flow.Header) {
	return func(header *flow.Header) {
		header.Height = height
	}
}

func HeaderWithView(view uint64) func(*flow.Header) {
	return func(header *flow.Header) {
		header.View = view
	}
}

func HeaderBodyFixture(opts ...func(header flow.HeaderBody)) flow.HeaderBody {
	height := 1 + uint64(rand.Uint32()) // avoiding edge case of height = 0 (genesis block)
	view := height + uint64(rand.Intn(1000))
	header := HeaderBodyWithParentFixture(&flow.Header{
		HeaderBody: flow.HeaderBody{
			ChainID:  flow.Emulator,
			ParentID: IdentifierFixture(),
			Height:   height,
			View:     view,
		},
	})

	for _, opt := range opts {
		opt(header)
	}

	return header
}

func BlockHeaderFixture(opts ...func(header *flow.Header)) *flow.Header {
	height := 1 + uint64(rand.Uint32()) // avoiding edge case of height = 0 (genesis block)
	view := height + uint64(rand.Intn(1000))
	header := BlockHeaderWithParentFixture(&flow.Header{
		HeaderBody: flow.HeaderBody{
			ChainID:            flow.Emulator,
			ParentID:           IdentifierFixture(),
			Height:             height,
			View:               view,
			ParentVoterIndices: SignerIndicesFixture(4),
			ParentVoterSigData: QCSigDataFixture(),
			ProposerID:         IdentifierFixture(),
		},
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
		HeaderBody: flow.HeaderBody{
			ChainID:  chainID,
			ParentID: IdentifierFixture(),
			Height:   height,
			View:     view,
		},
	})

	for _, opt := range opts {
		opt(header)
	}

	return header
}

func BlockHeaderWithParentFixture(parent *flow.Header) *flow.Header {
	return &flow.Header{
		HeaderBody:  HeaderBodyWithParentFixture(parent),
		PayloadHash: IdentifierFixture(),
	}
}

func HeaderBodyWithParentFixture(parent *flow.Header) flow.HeaderBody {
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
	return flow.HeaderBody{
		ChainID:            parent.ChainID,
		ParentID:           parent.ID(),
		Height:             height,
		Timestamp:          uint64(time.Now().UnixMilli()),
		View:               view,
		ParentView:         parent.View,
		ParentVoterIndices: SignerIndicesFixture(4),
		ParentVoterSigData: QCSigDataFixture(),
		ProposerID:         IdentifierFixture(),
		LastViewTC:         lastViewTC,
	}
}

func BlockHeaderWithHeight(height uint64) *flow.Header {
	return BlockHeaderFixture(WithHeaderHeight(height))
}

func BlockHeaderWithParentWithSoRFixture(parent *flow.Header, source []byte) *flow.Header {
	headerBody := HeaderBodyWithParentFixture(parent)
	headerBody.ParentVoterSigData = QCSigDataWithSoRFixture(source)

	return &flow.Header{
		HeaderBody:  headerBody,
		PayloadHash: IdentifierFixture(),
	}
}

func ClusterPayloadFixture(transactionsCount int) *cluster.Payload {
	return &cluster.Payload{
		ReferenceBlockID: IdentifierFixture(),
		Collection:       CollectionFixture(transactionsCount),
	}
}

func ClusterBlockFixtures(n int) []*cluster.Block {
	clusterBlocks := make([]*cluster.Block, 0, n)

	parent := ClusterBlockFixture()

	for i := 0; i < n; i++ {
		block := ClusterBlockFixture(
			ClusterBlock.WithParent(parent),
		)
		clusterBlocks = append(clusterBlocks, block)
		parent = block
	}

	return clusterBlocks
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

func AddCollectionsToBlock(block *flow.Block, collections []*flow.Collection) {
	gs := make([]*flow.CollectionGuarantee, 0, len(collections))
	for _, collection := range collections {
		gs = append(gs, &flow.CollectionGuarantee{CollectionID: collection.ID()})
	}

	block.Payload.Guarantees = gs
}

func CollectionGuaranteeFixture(options ...func(*flow.CollectionGuarantee)) *flow.CollectionGuarantee {
	guarantee := &flow.CollectionGuarantee{
		CollectionID:     IdentifierFixture(),
		ReferenceBlockID: IdentifierFixture(),
		ClusterChainID:   flow.ChainID("cluster-1-00000000"),
		SignerIndices:    RandomBytes(16),
		Signature:        SignatureFixture(),
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
		transactions = append(transactions, &tx)
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
		Collection: &flow.Collection{
			Transactions: []*flow.TransactionBody{&txBody},
		},
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
		Collection: &flow.Collection{
			Transactions: txs,
		},
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

	executableBlock := &entity.ExecutableBlock{
		Block:               &block,
		CompleteCollections: completeCollections,
	}
	// Preload the id
	executableBlock.BlockID()
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
		UnsignedExecutionReceipt: *UnsignedExecutionReceiptFixture(),
		ExecutorSignature:        SignatureFixture(),
	}

	for _, apply := range opts {
		apply(receipt)
	}

	return receipt
}

func UnsignedExecutionReceiptFixture(opts ...func(*flow.UnsignedExecutionReceipt)) *flow.UnsignedExecutionReceipt {
	receipt := &flow.UnsignedExecutionReceipt{
		ExecutorID:      IdentifierFixture(),
		ExecutionResult: *ExecutionResultFixture(),
		Spocks:          SignaturesFixture(1),
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

func WithPreviousResultID(previousResultID flow.Identifier) func(*flow.ExecutionResult) {
	return func(result *flow.ExecutionResult) {
		result.PreviousResultID = previousResultID
	}
}

func WithBlock(block *flow.Block) func(*flow.ExecutionResult) {
	chunks := 1 // tailing chunk is always system chunk
	chunks += len(block.Payload.Guarantees)
	blockID := block.ID()

	return func(result *flow.ExecutionResult) {
		startState := result.Chunks[0].StartState // retain previous start state in case it was user-defined
		result.BlockID = blockID
		result.Chunks = ChunkListFixture(uint(chunks), blockID, startState)
		result.PreviousResultID = IdentifierFixture()
	}
}

func WithChunks(n uint) func(*flow.ExecutionResult) {
	return func(result *flow.ExecutionResult) {
		result.Chunks = ChunkListFixture(n, result.BlockID, StateCommitmentFixture())
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
		// randomly assign service events to chunks
		for i := 0; i < n; i++ {
			chunkIndex := rand.Intn(result.Chunks.Len())
			result.Chunks[chunkIndex].ServiceEventCount++
		}
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
		Chunks:           ChunkListFixture(2, executedBlockID, StateCommitmentFixture()),
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
			Spock:                SignatureFixture(),
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
		bitutils.SetBit(indices, i)
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

// WithInitialWeight sets the initial weight on an identity fixture.
func WithInitialWeight(weight uint64) func(*flow.Identity) {
	return func(identity *flow.Identity) {
		identity.InitialWeight = weight
	}
}

// WithParticipationStatus sets the epoch participation status on an identity fixture.
func WithParticipationStatus(status flow.EpochParticipationStatus) func(*flow.Identity) {
	return func(identity *flow.Identity) {
		identity.EpochParticipationStatus = status
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
		Weight:  identity.InitialWeight,
	}
}

func NodeInfoFixture(opts ...func(*flow.Identity)) bootstrap.NodeInfo {
	nodes := NodeInfosFixture(1, opts...)
	return nodes[0]
}

// NodeInfoFromIdentity converts an identity to a public NodeInfo
// WARNING: the function replaces the staking key from the identity by a freshly generated one.
func NodeInfoFromIdentity(identity *flow.Identity) bootstrap.NodeInfo {
	stakingSK := StakingPrivKeyFixture()
	stakingPoP, err := crypto.BLSGeneratePOP(stakingSK)
	if err != nil {
		panic(err.Error())
	}

	return bootstrap.NewPublicNodeInfo(
		identity.NodeID,
		identity.Role,
		identity.Address,
		identity.InitialWeight,
		identity.NetworkPubKey,
		stakingSK.PublicKey(),
		stakingPoP,
	)
}

func NodeInfosFixture(n int, opts ...func(*flow.Identity)) []bootstrap.NodeInfo {
	opts = append(opts, WithKeys)
	il := IdentityListFixture(n, opts...)
	nodeInfos := make([]bootstrap.NodeInfo, 0, n)
	for _, identity := range il {
		nodeInfos = append(nodeInfos, NodeInfoFromIdentity(identity))
	}
	return nodeInfos
}

func PrivateNodeInfoFixture(opts ...func(*flow.Identity)) bootstrap.NodeInfo {
	return PrivateNodeInfosFixture(1, opts...)[0]
}

func PrivateNodeInfosFixture(n int, opts ...func(*flow.Identity)) []bootstrap.NodeInfo {
	return PrivateNodeInfosFromIdentityList(IdentityListFixture(n, opts...))
}

func PrivateNodeInfosFromIdentityList(il flow.IdentityList) []bootstrap.NodeInfo {
	nodeInfos := make([]bootstrap.NodeInfo, 0, len(il))
	for _, identity := range il {
		nodeInfo, err := bootstrap.PrivateNodeInfoFromIdentity(identity, KeyFixture(crypto.ECDSAP256), KeyFixture(crypto.BLSBLS12381))
		if err != nil {
			panic(err.Error())
		}
		nodeInfos = append(nodeInfos, nodeInfo)
	}
	return nodeInfos
}

// IdentityFixture returns a node identity.
func IdentityFixture(opts ...func(*flow.Identity)) *flow.Identity {
	nodeID := IdentifierFixture()
	stakingKey := StakingPrivKeyByIdentifier(nodeID)
	identity := flow.Identity{
		IdentitySkeleton: flow.IdentitySkeleton{
			NodeID:        nodeID,
			Address:       fmt.Sprintf("address-%x", nodeID[0:7]),
			Role:          flow.RoleConsensus,
			InitialWeight: 1000,
			StakingPubKey: stakingKey.PublicKey(),
		},
		DynamicIdentity: flow.DynamicIdentity{
			EpochParticipationStatus: flow.EpochParticipationStatusActive,
		},
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

// WithAllRoles can be used to ensure an IdentityList fixtures contains
// all the roles required for a valid genesis block.
func WithAllRoles() func(*flow.Identity) {
	return WithAllRolesExcept()
}

// WithAllRolesExcept is used to ensure an IdentityList fixture contains all roles
// except omitting a certain role, for cases where we are manually setting up nodes.
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

// DynamicIdentityEntryFixture returns the DynamicIdentityEntry object. The dynamic identity entry
// can be customized (ie. set Ejected).
func DynamicIdentityEntryFixture(opts ...func(*flow.DynamicIdentityEntry)) *flow.DynamicIdentityEntry {
	dynamicIdentityEntry := &flow.DynamicIdentityEntry{
		NodeID:  IdentifierFixture(),
		Ejected: false,
	}

	for _, opt := range opts {
		opt(dynamicIdentityEntry)
	}

	return dynamicIdentityEntry
}

// DynamicIdentityEntryListFixture returns a list of DynamicIdentityEntry objects.
func DynamicIdentityEntryListFixture(n int, opts ...func(*flow.DynamicIdentityEntry)) flow.DynamicIdentityEntryList {
	list := make(flow.DynamicIdentityEntryList, n)
	for i := 0; i < n; i++ {
		list[i] = DynamicIdentityEntryFixture(opts...)
	}
	return list
}

func WithChunkStartState(startState flow.StateCommitment) func(chunk *flow.Chunk) {
	return func(chunk *flow.Chunk) {
		chunk.StartState = startState
	}
}

func WithServiceEventCount(count uint16) func(*flow.Chunk) {
	return func(chunk *flow.Chunk) {
		chunk.ServiceEventCount = count
	}
}

func ChunkFixture(
	blockID flow.Identifier,
	collectionIndex uint,
	startState flow.StateCommitment,
	opts ...func(*flow.Chunk),
) *flow.Chunk {
	chunk := &flow.Chunk{
		ChunkBody: flow.ChunkBody{
			CollectionIndex:      collectionIndex,
			StartState:           startState,
			EventCollection:      IdentifierFixture(),
			ServiceEventCount:    0,
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

func ChunkListFixture(n uint, blockID flow.Identifier, startState flow.StateCommitment, opts ...func(*flow.Chunk)) flow.ChunkList {
	chunks := make([]*flow.Chunk, 0, n)
	for i := uint64(0); i < uint64(n); i++ {
		chunk := ChunkFixture(blockID, uint(i), startState, opts...)
		chunk.Index = i
		chunks = append(chunks, chunk)
		startState = chunk.EndState
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

func TransactionFixture(opts ...func(*flow.TransactionBody)) flow.TransactionBody {
	return TransactionBodyFixture(opts...)
}

// DEPRECATED: please use TransactionFixture instead
func TransactionBodyFixture(opts ...func(*flow.TransactionBody)) flow.TransactionBody {
	tb := flow.TransactionBody{
		Script:             []byte("access(all) fun main() {}"),
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
		l[i] = TransactionFixture()
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
		Imports: dsl.Imports{
			dsl.Import{
				Address: sdk.Address(chain.ServiceAddress()),
				Names:   []string{"FlowTransactionScheduler"},
			},
		},
		Content: dsl.Prepare{
			Content: dsl.Code(`
				access(all) fun main() {}
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
func VerifiableChunkDataFixture(chunkIndex uint64, opts ...func(*flow.HeaderBody)) (*verification.VerifiableChunkData, *flow.Block) {

	guarantees := make([]*flow.CollectionGuarantee, 0, chunkIndex+1)

	var col flow.Collection

	for i := 0; i <= int(chunkIndex); i++ {
		col = CollectionFixture(1)
		guarantees = append(guarantees, &flow.CollectionGuarantee{CollectionID: col.ID()})
	}

	payload := flow.Payload{
		Guarantees: guarantees,
		Seals:      nil,
	}
	headerBody := HeaderBodyFixture()
	for _, opt := range opts {
		opt(&headerBody)
	}

	block := &flow.Block{
		HeaderBody: headerBody,
		Payload:    payload,
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
		PreviousResultID: IdentifierFixture(),
		BlockID:          block.ID(),
		Chunks:           chunks,
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

	chunkDataPack := ChunkDataPackFixture(chunk.ID(), func(c *flow.ChunkDataPack) {
		c.Collection = &col
	})

	return &verification.VerifiableChunkData{
		Chunk:         &chunk,
		Header:        block.ToHeader(),
		Result:        &result,
		ChunkDataPack: chunkDataPack,
		EndState:      endState,
	}, block
}

// ChunkDataResponseMsgFixture creates a chunk data response message with a single-transaction collection, and random chunk ID.
// Use options to customize the response.
func ChunkDataResponseMsgFixture(
	chunkID flow.Identifier,
	opts ...func(*messages.ChunkDataResponse),
) *messages.ChunkDataResponse {
	cdp := &messages.ChunkDataResponse{
		ChunkDataPack: flow.UntrustedChunkDataPack(*ChunkDataPackFixture(chunkID)),
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
		request.ChunkDataPack = flow.UntrustedChunkDataPack(*ChunkDataPackFixture(request.ChunkDataPack.ChunkID, WithChunkDataPackCollection(&collection)))
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
		Locator:                  *ChunkLocatorFixture(IdentifierFixture(), 0),
		ChunkDataPackRequestInfo: *ChunkDataPackRequestInfoFixture(),
	}

	for _, opt := range opts {
		opt(req)
	}

	// Ensure Targets reflects current Agrees and Disagrees
	req.Targets = makeTargets(req.Agrees, req.Disagrees)

	return req
}

func ChunkDataPackRequestInfoFixture() *verification.ChunkDataPackRequestInfo {
	agrees := IdentifierListFixture(1)
	disagrees := IdentifierListFixture(1)

	return &verification.ChunkDataPackRequestInfo{
		ChunkID:   IdentifierFixture(),
		Height:    0,
		Agrees:    agrees,
		Disagrees: disagrees,
		Targets:   makeTargets(agrees, disagrees),
	}
}

// makeTargets returns a combined IdentityList for the given agrees and disagrees.
func makeTargets(agrees, disagrees flow.IdentifierList) flow.IdentityList {
	// creates identity fixtures for target ids as union of agrees and disagrees
	// TODO: remove this inner fixture once we have filter for identifier list.
	targets := flow.IdentityList{}
	for _, id := range append(agrees, disagrees...) {
		targets = append(targets, IdentityFixture(
			WithNodeID(id),
			WithRole(flow.RoleExecution),
		))
	}
	return targets
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

func ChunkDataPacksFixtureAndResult() ([]*flow.ChunkDataPack, *flow.ExecutionResult) {
	result := ExecutionResultFixture()
	cdps := make([]*flow.ChunkDataPack, 0, len(result.Chunks))
	for _, c := range result.Chunks {
		cdps = append(cdps, ChunkDataPackFixture(c.ID()))
	}
	return cdps, result
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
) flow.BlockEvents {
	return flow.BlockEvents{
		BlockID:        header.ID(),
		BlockHeight:    header.Height,
		BlockTimestamp: time.UnixMilli(int64(header.Timestamp)).UTC(),
		Events:         EventsFixture(n),
	}
}

func EventTypeFixture(chainID flow.ChainID) flow.EventType {
	eventType := fmt.Sprintf("A.%s.TestContract.TestEvent1", RandomAddressFixtureForChain(chainID))
	return flow.EventType(eventType)
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

func CertifiedByChild(block *flow.Block, child *flow.Block) *flow.CertifiedBlock {
	return &flow.CertifiedBlock{
		Proposal:     &flow.Proposal{Block: *block, ProposerSigData: SignatureFixture()},
		CertifyingQC: child.ParentQC(),
	}
}

func NewCertifiedBlock(block *flow.Block) *flow.CertifiedBlock {
	return &flow.CertifiedBlock{
		Proposal: &flow.Proposal{
			Block:           *block,
			ProposerSigData: SignatureFixture(),
		},
		CertifyingQC: CertifyBlock(block.ToHeader()),
	}
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

func WithParticipants(participants flow.IdentitySkeletonList) func(*flow.EpochSetup) {
	return func(setup *flow.EpochSetup) {
		setup.Participants = participants.Sort(flow.Canonical[flow.IdentitySkeleton])
		setup.Assignments = ClusterAssignment(1, participants.ToSkeleton())
	}
}

func WithAssignments(assignments flow.AssignmentList) func(*flow.EpochSetup) {
	return func(setup *flow.EpochSetup) {
		setup.Assignments = assignments
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
		Participants:       participants.Sort(flow.Canonical[flow.Identity]).ToSkeleton(),
		RandomSource:       SeedFixture(flow.EpochSetupRandomSourceLength),
		DKGPhase1FinalView: 100,
		DKGPhase2FinalView: 200,
		DKGPhase3FinalView: 300,
		TargetDuration:     60 * 60,
		TargetEndTime:      uint64(time.Now().Add(time.Hour).Unix()),
	}
	for _, apply := range opts {
		apply(setup)
	}
	if setup.Assignments == nil {
		setup.Assignments = ClusterAssignment(1, setup.Participants)
	}
	return setup
}

// EpochRecoverFixture creates a valid EpochRecover with default properties for testing.
// The default properties for setup part can be overwritten with optional parameter functions.
// Commit part will be adjusted accordingly.
func EpochRecoverFixture(opts ...func(setup *flow.EpochSetup)) *flow.EpochRecover {
	setup := EpochSetupFixture()
	for _, apply := range opts {
		apply(setup)
	}

	commit := EpochCommitFixture(
		CommitWithCounter(setup.Counter),
		WithDKGFromParticipants(setup.Participants),
		WithClusterQCsFromAssignments(setup.Assignments),
	)

	return &flow.EpochRecover{
		EpochSetup:  *setup,
		EpochCommit: *commit,
	}
}

func IndexFixture() *flow.Index {
	return &flow.Index{
		GuaranteeIDs: IdentifierListFixture(5),
		SealIDs:      IdentifierListFixture(5),
		ReceiptIDs:   IdentifierListFixture(5),
	}
}

func WithDKGFromParticipants(participants flow.IdentitySkeletonList) func(*flow.EpochCommit) {
	dkgParticipants := participants.Filter(filter.IsConsensusCommitteeMember).Sort(flow.Canonical[flow.IdentitySkeleton])
	return func(commit *flow.EpochCommit) {
		commit.DKGParticipantKeys = nil
		commit.DKGIndexMap = make(flow.DKGIndexMap)
		for index, nodeID := range dkgParticipants.NodeIDs() {
			commit.DKGParticipantKeys = append(commit.DKGParticipantKeys, KeyFixture(crypto.BLSBLS12381).PublicKey())
			commit.DKGIndexMap[nodeID] = index
		}
	}
}

func WithClusterQCs(qcs []flow.ClusterQCVoteData) func(*flow.EpochCommit) {
	return func(commit *flow.EpochCommit) {
		commit.ClusterQCs = qcs
	}
}

func WithClusterQCsFromAssignments(assignments flow.AssignmentList) func(*flow.EpochCommit) {
	qcs := QuorumCertificatesFromAssignments(assignments)
	return func(commit *flow.EpochCommit) {
		commit.ClusterQCs = flow.ClusterQCVoteDatasFromQCs(qcs)
	}
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

func ProtocolStateVersionUpgradeFixture() *flow.ProtocolStateVersionUpgrade {
	return &flow.ProtocolStateVersionUpgrade{
		NewProtocolStateVersion: rand.Uint64(),
		ActiveView:              rand.Uint64(),
	}
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
	root := Block.Genesis(chainID)
	for _, apply := range opts {
		apply(root)
	}

	counter := uint64(1)
	setup := EpochSetupFixture(
		WithParticipants(participants.ToSkeleton()),
		SetupWithCounter(counter),
		WithFirstView(root.View),
		WithFinalView(root.View+100_000),
	)
	commit := EpochCommitFixture(
		CommitWithCounter(counter),
		WithClusterQCsFromAssignments(setup.Assignments),
		WithDKGFromParticipants(participants.ToSkeleton()),
	)

	return BootstrapFixtureWithSetupAndCommit(root.HeaderBody, setup, commit)
}

// BootstrapFixtureWithSetupAndCommit generates all the artifacts necessary to bootstrap the
// protocol state using the provided epoch setup and commit.
func BootstrapFixtureWithSetupAndCommit(
	header flow.HeaderBody,
	setup *flow.EpochSetup,
	commit *flow.EpochCommit,
) (*flow.Block, *flow.ExecutionResult, *flow.Seal) {
	safetyParams, err := protocol.DefaultEpochSafetyParams(header.ChainID)
	if err != nil {
		panic(err)
	}
	rootEpochState, err := inmem.EpochProtocolStateFromServiceEvents(setup, commit)
	if err != nil {
		panic(err)
	}
	rootProtocolState, err := kvstore.NewDefaultKVStore(safetyParams.FinalizationSafetyThreshold, safetyParams.EpochExtensionViewCount, rootEpochState.ID())
	if err != nil {
		panic(err)
	}

	root := &flow.Block{
		HeaderBody: header,
		Payload:    flow.Payload{ProtocolStateID: rootProtocolState.ID()},
	}

	stateCommit := GenesisStateCommitmentByChainID(header.ChainID)

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
	block, result, seal := BootstrapFixtureWithChainID(participants.Sort(flow.Canonical[flow.Identity]), chainID, opts...)
	qc := QuorumCertificateFixture(QCWithRootBlockID(block.ID()))
	root, err := SnapshotFromBootstrapState(block, result, seal, qc)
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
	epoch, err := epochs.Current()
	if err != nil {
		return nil, err
	}
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

	children := ChainFixtureFrom(nonGenesisCount, genesis.ToHeader())
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
		parent = block.ToHeader()
	}

	return blocks
}

// ProposalChainFixtureFrom creates a chain of blocks and wraps each one in a Proposal.
func ProposalChainFixtureFrom(count int, parent *flow.Header) []*flow.Proposal {
	proposals := make([]*flow.Proposal, 0, count)
	for _, block := range ChainFixtureFrom(count, parent) {
		proposals = append(proposals, ProposalFromBlock(block))
	}
	return proposals
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
		if prev.Height+1 != b.Height {
			panic(fmt.Sprintf("height has gap when connecting blocks: expect %v, but got %v", prev.Height+1, b.Height))
		}
		b.ParentID = prev.ID()
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

// PrivateKeyFixture returns a random private key with specified signature algorithm
func PrivateKeyFixture(algo crypto.SigningAlgorithm) crypto.PrivateKey {
	sk, err := crypto.GeneratePrivateKey(algo, SeedFixture(crypto.KeyGenSeedMinLen))
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
	seed := make([]byte, seedLength)
	copy(seed, id[:])
	sk, err := crypto.GeneratePrivateKey(algo, seed)
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
	return PrivateKeyFixture(crypto.ECDSAP256)
}

// StakingPrivKeyFixture returns a random BLS12381 private keyf
func StakingPrivKeyFixture() crypto.PrivateKey {
	return PrivateKeyFixture(crypto.BLSBLS12381)
}

func NodeMachineAccountInfoFixture() bootstrap.NodeMachineAccountInfo {
	return bootstrap.NodeMachineAccountInfo{
		Address:           RandomAddressFixture().String(),
		EncodedPrivateKey: PrivateKeyFixture(crypto.ECDSAP256).Encode(),
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

	bal, err := cadence.NewUFix64("5.0")
	require.NoError(t, err)

	acct := &sdk.Account{
		Address: sdk.HexToAddress(info.Address),
		Balance: uint64(bal),
		Keys: []*sdk.AccountKey{
			{
				Index:     info.KeyIndex,
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

func WithTxResultErrorMessageTxID(id flow.Identifier) func(txResErrMsg *flow.TransactionResultErrorMessage) {
	return func(txResErrMsg *flow.TransactionResultErrorMessage) {
		txResErrMsg.TransactionID = id
	}
}

func WithTxResultErrorMessageIndex(index uint32) func(txResErrMsg *flow.TransactionResultErrorMessage) {
	return func(txResErrMsg *flow.TransactionResultErrorMessage) {
		txResErrMsg.Index = index
	}
}

func WithTxResultErrorMessageTxMsg(message string) func(txResErrMsg *flow.TransactionResultErrorMessage) {
	return func(txResErrMsg *flow.TransactionResultErrorMessage) {
		txResErrMsg.ErrorMessage = message
	}
}

func WithTxResultErrorMessageExecutorID(id flow.Identifier) func(txResErrMsg *flow.TransactionResultErrorMessage) {
	return func(txResErrMsg *flow.TransactionResultErrorMessage) {
		txResErrMsg.ExecutorID = id
	}
}

// TransactionResultErrorMessageFixture creates a fixture tx result error message with random generated tx ID and executor ID for test purpose.
func TransactionResultErrorMessageFixture(opts ...func(*flow.TransactionResultErrorMessage)) flow.TransactionResultErrorMessage {
	txResErrMsg := flow.TransactionResultErrorMessage{
		TransactionID: IdentifierFixture(),
		Index:         0,
		ErrorMessage:  "transaction result error",
		ExecutorID:    IdentifierFixture(),
	}

	for _, opt := range opts {
		opt(&txResErrMsg)
	}

	return txResErrMsg
}

// TransactionResultErrorMessagesFixture creates a fixture collection of tx result error messages with n elements.
func TransactionResultErrorMessagesFixture(n int) []flow.TransactionResultErrorMessage {
	txResErrMsgs := make([]flow.TransactionResultErrorMessage, 0, n)
	executorID := IdentifierFixture()

	for i := 0; i < n; i++ {
		txResErrMsgs = append(txResErrMsgs, TransactionResultErrorMessageFixture(
			WithTxResultErrorMessageIndex(uint32(i)),
			WithTxResultErrorMessageTxMsg(fmt.Sprintf("transaction result error %d", i)),
			WithTxResultErrorMessageExecutorID(executorID),
		))
	}
	return txResErrMsgs
}

// RootEpochProtocolStateFixture creates a fixture with correctly structured Epoch sub-state.
// The epoch substate is part of the overall protocol state (KV store).
// This can be useful for testing bootstrap when there is no previous epoch.
func RootEpochProtocolStateFixture() *flow.RichEpochStateEntry {
	currentEpochSetup := EpochSetupFixture(func(setup *flow.EpochSetup) {
		setup.Counter = 1
	})
	currentEpochCommit := EpochCommitFixture(func(commit *flow.EpochCommit) {
		commit.Counter = currentEpochSetup.Counter
	})

	allIdentities := make(flow.IdentityList, 0, len(currentEpochSetup.Participants))
	for _, identity := range currentEpochSetup.Participants {
		allIdentities = append(allIdentities, &flow.Identity{
			IdentitySkeleton: *identity,
			DynamicIdentity: flow.DynamicIdentity{
				EpochParticipationStatus: flow.EpochParticipationStatusActive,
			},
		})
	}
	return &flow.RichEpochStateEntry{
		EpochStateEntry: &flow.EpochStateEntry{
			MinEpochStateEntry: &flow.MinEpochStateEntry{
				PreviousEpoch: nil,
				CurrentEpoch: flow.EpochStateContainer{
					SetupID:          currentEpochSetup.ID(),
					CommitID:         currentEpochCommit.ID(),
					ActiveIdentities: flow.DynamicIdentityEntryListFromIdentities(allIdentities),
				},
				EpochFallbackTriggered: false,
				NextEpoch:              nil,
			},
			PreviousEpochSetup:  nil,
			PreviousEpochCommit: nil,
			CurrentEpochSetup:   currentEpochSetup,
			CurrentEpochCommit:  currentEpochCommit,
			NextEpochSetup:      nil,
			NextEpochCommit:     nil,
		},
		CurrentEpochIdentityTable: allIdentities,
		NextEpochIdentityTable:    flow.IdentityList{},
	}
}

// EpochStateFixture creates a fixture with correctly structured data. The returned Identity Table
// represents the common situation during the staking phase of Epoch N+1:
//   - we are currently in Epoch N
//   - previous epoch N-1 is known (specifically EpochSetup and EpochCommit events)
//   - network is currently in the staking phase to setup the next epoch, hence no service
//     events for the next epoch exist
//
// In particular, the following consistency requirements hold:
//   - Epoch setup and commit counters are set to match.
//   - Identities are constructed from setup events.
//   - Identities are sorted in canonical order.
func EpochStateFixture(options ...func(*flow.RichEpochStateEntry)) *flow.RichEpochStateEntry {
	prevEpochSetup := EpochSetupFixture()
	prevEpochCommit := EpochCommitFixture(func(commit *flow.EpochCommit) {
		commit.Counter = prevEpochSetup.Counter
	})
	currentEpochSetup := EpochSetupFixture(func(setup *flow.EpochSetup) {
		setup.Counter = prevEpochSetup.Counter + 1
		// reuse same participant for current epoch
		sameParticipant := *prevEpochSetup.Participants[1]
		setup.Participants = append(setup.Participants, &sameParticipant)
		setup.Participants = setup.Participants.Sort(flow.Canonical[flow.IdentitySkeleton])
	})
	currentEpochCommit := EpochCommitFixture(func(commit *flow.EpochCommit) {
		commit.Counter = currentEpochSetup.Counter
	})

	buildDefaultIdentities := func(setup *flow.EpochSetup) flow.IdentityList {
		epochIdentities := make(flow.IdentityList, 0, len(setup.Participants))
		for _, identity := range setup.Participants {
			epochIdentities = append(epochIdentities, &flow.Identity{
				IdentitySkeleton: *identity,
				DynamicIdentity: flow.DynamicIdentity{
					EpochParticipationStatus: flow.EpochParticipationStatusActive,
				},
			})
		}
		return epochIdentities.Sort(flow.Canonical[flow.Identity])
	}

	prevEpochIdentities := buildDefaultIdentities(prevEpochSetup)
	currentEpochIdentities := buildDefaultIdentities(currentEpochSetup)
	allIdentities := currentEpochIdentities.Union(
		prevEpochIdentities.Map(mapfunc.WithEpochParticipationStatus(flow.EpochParticipationStatusLeaving)))

	entry := &flow.RichEpochStateEntry{
		EpochStateEntry: &flow.EpochStateEntry{
			MinEpochStateEntry: &flow.MinEpochStateEntry{
				CurrentEpoch: flow.EpochStateContainer{
					SetupID:          currentEpochSetup.ID(),
					CommitID:         currentEpochCommit.ID(),
					ActiveIdentities: flow.DynamicIdentityEntryListFromIdentities(currentEpochIdentities),
				},
				PreviousEpoch: &flow.EpochStateContainer{
					SetupID:          prevEpochSetup.ID(),
					CommitID:         prevEpochCommit.ID(),
					ActiveIdentities: flow.DynamicIdentityEntryListFromIdentities(prevEpochIdentities),
				},
				EpochFallbackTriggered: false,
				NextEpoch:              nil,
			},
			PreviousEpochSetup:  prevEpochSetup,
			PreviousEpochCommit: prevEpochCommit,
			CurrentEpochSetup:   currentEpochSetup,
			CurrentEpochCommit:  currentEpochCommit,
			NextEpochSetup:      nil,
			NextEpochCommit:     nil,
		},
		CurrentEpochIdentityTable: allIdentities,
		NextEpochIdentityTable:    flow.IdentityList{},
	}

	for _, option := range options {
		option(entry)
	}

	return entry
}

// WithNextEpochProtocolState creates a fixture with correctly structured data for next epoch.
// The resulting Identity Table represents the common situation during the epoch commit phase for Epoch N+1:
//   - We are currently in Epoch N.
//   - The previous epoch N-1 is known (specifically EpochSetup and EpochCommit events).
//   - The network has completed the epoch setup phase, i.e. published the EpochSetup and EpochCommit events for epoch N+1.
func WithNextEpochProtocolState() func(entry *flow.RichEpochStateEntry) {
	return func(entry *flow.RichEpochStateEntry) {
		nextEpochSetup := EpochSetupFixture(func(setup *flow.EpochSetup) {
			setup.Counter = entry.CurrentEpochSetup.Counter + 1
			setup.FirstView = entry.CurrentEpochSetup.FinalView + 1
			setup.FinalView = setup.FirstView + 1000
			// reuse same participant for current epoch
			sameParticipant := *entry.CurrentEpochSetup.Participants[1]
			setup.Participants[1] = &sameParticipant
			setup.Participants = setup.Participants.Sort(flow.Canonical[flow.IdentitySkeleton])
		})
		nextEpochCommit := EpochCommitFixture(func(commit *flow.EpochCommit) {
			commit.Counter = nextEpochSetup.Counter
		})

		nextEpochParticipants := make(flow.IdentityList, 0, len(nextEpochSetup.Participants))
		for _, identity := range nextEpochSetup.Participants {
			nextEpochParticipants = append(nextEpochParticipants, &flow.Identity{
				IdentitySkeleton: *identity,
				DynamicIdentity: flow.DynamicIdentity{
					EpochParticipationStatus: flow.EpochParticipationStatusActive,
				},
			})
		}
		nextEpochParticipants = nextEpochParticipants.Sort(flow.Canonical[flow.Identity])

		currentEpochParticipants := entry.CurrentEpochIdentityTable.Filter(func(identity *flow.Identity) bool {
			_, found := entry.CurrentEpochSetup.Participants.ByNodeID(identity.NodeID)
			return found
		}).Sort(flow.Canonical[flow.Identity])

		entry.CurrentEpochIdentityTable = currentEpochParticipants.Union(
			nextEpochParticipants.Map(mapfunc.WithEpochParticipationStatus(flow.EpochParticipationStatusJoining)))
		entry.NextEpochIdentityTable = nextEpochParticipants.Union(
			currentEpochParticipants.Map(mapfunc.WithEpochParticipationStatus(flow.EpochParticipationStatusLeaving)))

		entry.NextEpoch = &flow.EpochStateContainer{
			SetupID:          nextEpochSetup.ID(),
			CommitID:         nextEpochCommit.ID(),
			ActiveIdentities: flow.DynamicIdentityEntryListFromIdentities(nextEpochParticipants),
		}
		entry.NextEpochSetup = nextEpochSetup
		entry.NextEpochCommit = nextEpochCommit
	}
}

// WithValidDKG updated protocol state with correctly structured data for DKG.
func WithValidDKG() func(*flow.RichEpochStateEntry) {
	return func(entry *flow.RichEpochStateEntry) {
		commit := entry.CurrentEpochCommit
		dkgParticipants := entry.CurrentEpochSetup.Participants.Filter(filter.IsConsensusCommitteeMember).Sort(flow.Canonical[flow.IdentitySkeleton])
		commit.DKGParticipantKeys = nil
		commit.DKGIndexMap = make(flow.DKGIndexMap)
		for index, nodeID := range dkgParticipants.NodeIDs() {
			commit.DKGParticipantKeys = append(commit.DKGParticipantKeys, KeyFixture(crypto.BLSBLS12381).PublicKey())
			commit.DKGIndexMap[nodeID] = index
		}
		// update CommitID according to new CurrentEpochCommit object
		entry.MinEpochStateEntry.CurrentEpoch.CommitID = entry.CurrentEpochCommit.ID()
	}
}

// EpochProtocolStateEntryFixture returns a flow.MinEpochStateEntry fixture.
//   - PreviousEpoch is always nil
//   - tentativePhase defines what service events should be defined for the NextEpoch
//   - efmTriggered defines whether the EpochFallbackTriggered flag should be set.
func EpochProtocolStateEntryFixture(tentativePhase flow.EpochPhase, efmTriggered bool) flow.MinEpochStateEntry {
	identities := DynamicIdentityEntryListFixture(5)
	entry := flow.MinEpochStateEntry{
		EpochFallbackTriggered: efmTriggered,
		PreviousEpoch:          nil,
		CurrentEpoch: flow.EpochStateContainer{
			SetupID:          IdentifierFixture(),
			CommitID:         IdentifierFixture(),
			ActiveIdentities: identities,
		},
		NextEpoch: nil,
	}

	switch tentativePhase {
	case flow.EpochPhaseStaking:
		break
	case flow.EpochPhaseSetup:
		entry.NextEpoch = &flow.EpochStateContainer{
			SetupID:          IdentifierFixture(),
			CommitID:         flow.ZeroID,
			ActiveIdentities: identities,
		}
	case flow.EpochPhaseCommitted:
		entry.NextEpoch = &flow.EpochStateContainer{
			SetupID:          IdentifierFixture(),
			CommitID:         IdentifierFixture(),
			ActiveIdentities: identities,
		}
	default:
		panic("unexpected input phase: " + tentativePhase.String())
	}
	return entry
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

// ViewBasedActivatorFixture returns a ViewBasedActivator with randomly generated Data and ActivationView.
func ViewBasedActivatorFixture() *protocol.ViewBasedActivator[uint64] {
	return &protocol.ViewBasedActivator[uint64]{
		Data:           rand.Uint64(),
		ActivationView: rand.Uint64(),
	}
}

// EpochExtensionFixture returns a randomly generated EpochExtensionFixture object.
func EpochExtensionFixture() flow.EpochExtension {
	firstView := rand.Uint64()

	return flow.EpochExtension{
		FirstView: firstView,
		FinalView: firstView + uint64(rand.Intn(10)+1),
	}
}

// EpochStateContainerFixture returns a randomly generated EpochStateContainer object.
func EpochStateContainerFixture() *flow.EpochStateContainer {
	return &flow.EpochStateContainer{
		SetupID:          IdentifierFixture(),
		CommitID:         IdentifierFixture(),
		ActiveIdentities: DynamicIdentityEntryListFixture(5),
		EpochExtensions:  []flow.EpochExtension{EpochExtensionFixture()},
	}
}

func EpochSetupRandomSourceFixture() []byte {
	source := make([]byte, flow.EpochSetupRandomSourceLength)
	_, err := rand.Read(source)
	if err != nil {
		panic(err)
	}
	return source
}

// SnapshotFromBootstrapState generates a protocol.Snapshot representing a
// root bootstrap state. This is used to bootstrap the protocol state for
// genesis or post-spork states.
func SnapshotFromBootstrapState(root *flow.Block, result *flow.ExecutionResult, seal *flow.Seal, qc *flow.QuorumCertificate) (*inmem.Snapshot, error) {
	safetyParams, err := protocol.DefaultEpochSafetyParams(root.ChainID)
	if err != nil {
		return nil, fmt.Errorf("could not get default epoch commit safety threshold: %w", err)
	}
	return inmem.SnapshotFromBootstrapStateWithParams(root, result, seal, qc, func(epochStateID flow.Identifier) (protocol_state.KVStoreAPI, error) {
		return kvstore.NewDefaultKVStore(safetyParams.FinalizationSafetyThreshold, safetyParams.EpochExtensionViewCount, epochStateID)
	})
}
