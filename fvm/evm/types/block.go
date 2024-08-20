package types

import (
	"bytes"
	"math/big"
	"time"

	gethCommon "github.com/onflow/go-ethereum/common"
	gethTypes "github.com/onflow/go-ethereum/core/types"
	gethCrypto "github.com/onflow/go-ethereum/crypto"
	gethRLP "github.com/onflow/go-ethereum/rlp"
	gethTrie "github.com/onflow/go-ethereum/trie"

	"github.com/onflow/flow-go/model/flow"
)

// Block represents a evm block.
// It captures block info such as height and state
type Block struct {
	// the hash of the parent block
	ParentBlockHash gethCommon.Hash

	// Height returns the height of this block
	Height uint64

	// Timestamp is a Unix timestamp in seconds at which the block was created
	// Note that this value must be provided from the FVM Block
	Timestamp uint64

	// holds the total amount of the native token deposited in the evm side. (in attoflow)
	TotalSupply *big.Int

	// ReceiptRoot returns the root hash of the receipts emitted in this block
	// Note that this value won't be unique to each block, for example for the
	// case of empty trie of receipts or a single receipt with no logs and failed state
	// the same receipt root would be reported for block.
	ReceiptRoot gethCommon.Hash

	// TransactionHashRoot returns the root hash of the transaction hashes
	// included in this block.
	// Note that despite similar functionality this is a bit different than TransactionRoot
	// provided by Ethereum. TransactionRoot constructs a Merkle proof with leafs holding
	// encoded transactions as values. But TransactionHashRoot uses transaction hash
	// values as node values. Proofs are still compatible but might require an extra hashing step.
	TransactionHashRoot gethCommon.Hash

	// TotalGasUsed stores gas used by all transactions included in the block.
	TotalGasUsed uint64

	// Prevrandao is the value returned for Prevrandao
	Prevrandao gethCommon.Hash
}

// ToBytes encodes the block into bytes
func (b *Block) ToBytes() ([]byte, error) {
	return gethRLP.EncodeToBytes(b)
}

// Hash returns the hash of the block
func (b *Block) Hash() (gethCommon.Hash, error) {
	data, err := b.ToBytes()
	return gethCrypto.Keccak256Hash(data), err
}

// NewBlock constructs a new block
func NewBlock(
	parentBlockHash gethCommon.Hash,
	height uint64,
	timestamp uint64,
	totalSupply *big.Int,
	prevrandao gethCommon.Hash,
) *Block {
	return &Block{
		ParentBlockHash:     parentBlockHash,
		Height:              height,
		Timestamp:           timestamp,
		TotalSupply:         totalSupply,
		ReceiptRoot:         gethTypes.EmptyReceiptsHash,
		TransactionHashRoot: gethTypes.EmptyRootHash,
		Prevrandao:          prevrandao,
	}
}

// NewBlockFromBytes constructs a new block from encoded data
func NewBlockFromBytes(encoded []byte) (*Block, error) {
	res := &Block{}
	err := gethRLP.DecodeBytes(encoded, res)
	if err != nil {
		res = decodeBlockBreakingChanges(encoded)
		if res == nil {
			return nil, err
		}
	}
	return res, nil
}

// GenesisTimestamp returns the block time stamp for EVM genesis block
func GenesisTimestamp(flowChainID flow.ChainID) uint64 {
	switch flowChainID {
	case flow.Testnet:
		return uint64(time.Date(2024, time.August, 1, 0, 0, 0, 0, time.UTC).Unix())
	case flow.Mainnet:
		return uint64(time.Date(2024, time.September, 1, 0, 0, 0, 0, time.UTC).Unix())
	default:
		return 0
	}
}

// GenesisBlock returns the genesis block in the EVM environment
func GenesisBlock(chainID flow.ChainID) *Block {
	return &Block{
		ParentBlockHash:     gethCommon.Hash{},
		Height:              uint64(0),
		Timestamp:           GenesisTimestamp(chainID),
		TotalSupply:         new(big.Int),
		ReceiptRoot:         gethTypes.EmptyRootHash,
		TransactionHashRoot: gethTypes.EmptyRootHash,
		TotalGasUsed:        0,
		Prevrandao:          gethCommon.Hash{},
	}
}

// GenesisBlockHash returns the genesis block hash in the EVM environment
func GenesisBlockHash(chainID flow.ChainID) gethCommon.Hash {
	h, err := GenesisBlock(chainID).Hash()
	if err != nil { // this never happens
		panic(err)
	}
	return h
}

// BlockProposal is a EVM block proposal
// holding all the interim data of block before commitment
type BlockProposal struct {
	Block

	// Receipts keeps a order list of light receipts generated during block execution
	Receipts []LightReceipt

	// TxHashes keeps transaction hashes included in this block proposal
	TxHashes TransactionHashes
}

// AppendTransaction appends a transaction hash to the list of transaction hashes of the block
// and also update the receipts
func (b *BlockProposal) AppendTransaction(res *Result) {
	// we don't append invalid transactions to blocks
	if res == nil || res.Invalid() {
		return
	}
	b.TxHashes = append(b.TxHashes, res.TxHash)
	r := res.LightReceipt()
	if r == nil {
		return
	}
	b.Receipts = append(b.Receipts, *r)
	b.TotalGasUsed = r.CumulativeGasUsed
}

// PopulateRoots populates receiptRoot and transactionHashRoot
func (b *BlockProposal) PopulateRoots() {
	// TODO: we can make this concurrent if needed in the future
	// to improve the block production speed
	b.PopulateTransactionHashRoot()
	b.PopulateReceiptRoot()
}

// PopulateTransactionHashRoot sets the transactionHashRoot
func (b *BlockProposal) PopulateTransactionHashRoot() {
	if len(b.TransactionHashRoot) == 0 {
		b.TransactionHashRoot = gethTypes.EmptyRootHash
		return
	}
	b.TransactionHashRoot = b.TxHashes.RootHash()
}

// PopulateReceiptRoot sets the receiptRoot
func (b *BlockProposal) PopulateReceiptRoot() {
	if len(b.Receipts) == 0 {
		b.ReceiptRoot = gethTypes.EmptyReceiptsHash
		return
	}
	receipts := make(gethTypes.Receipts, len(b.Receipts))
	for i, lr := range b.Receipts {
		receipts[i] = lr.ToReceipt()
	}
	b.ReceiptRoot = gethTypes.DeriveSha(receipts, gethTrie.NewStackTrie(nil))
}

// ToBytes encodes the block proposal into bytes
func (b *BlockProposal) ToBytes() ([]byte, error) {
	return gethRLP.EncodeToBytes(b)
}

// NewBlockProposalFromBytes constructs a new block proposal from encoded data
func NewBlockProposalFromBytes(encoded []byte) (*BlockProposal, error) {
	res := &BlockProposal{}
	err := gethRLP.DecodeBytes(encoded, res)
	if err != nil {
		res = decodeBlockProposalBreakingChanges(encoded)
		if res == nil {
			return nil, err
		}
	}
	return res, nil
}

func NewBlockProposal(
	parentBlockHash gethCommon.Hash,
	height uint64,
	timestamp uint64,
	totalSupply *big.Int,
	prevrandao gethCommon.Hash,
) *BlockProposal {
	return &BlockProposal{
		Block: Block{
			ParentBlockHash: parentBlockHash,
			Height:          height,
			Timestamp:       timestamp,
			TotalSupply:     totalSupply,
			ReceiptRoot:     gethTypes.EmptyRootHash,
			Prevrandao:      prevrandao,
		},
		Receipts: make([]LightReceipt, 0),
		TxHashes: make([]gethCommon.Hash, 0),
	}
}

type TransactionHashes []gethCommon.Hash

func (t TransactionHashes) Len() int {
	return len(t)
}

func (t TransactionHashes) EncodeIndex(index int, buffer *bytes.Buffer) {
	buffer.Write(t[index].Bytes())
}

func (t TransactionHashes) RootHash() gethCommon.Hash {
	return gethTypes.DeriveSha(t, gethTrie.NewStackTrie(nil))
}

// Below block type section, defines earlier block types,
// this is being used to decode blocks that were stored
// before block type changes. It allows us to still decode
// a block that would otherwise be invalid if decoded into
// latest version of the above Block type.

// before adding Prevrandao to block
type BlockV0 struct {
	ParentBlockHash     gethCommon.Hash
	Height              uint64
	Timestamp           uint64
	TotalSupply         *big.Int
	ReceiptRoot         gethCommon.Hash
	TransactionHashRoot gethCommon.Hash
	TotalGasUsed        uint64
}

type BlockProposalV0 struct {
	BlockV0
	Receipts []LightReceipt
	TxHashes TransactionHashes
}

// decodeBlockBreakingChanges will try to decode the bytes into all
// previous versions of block type, if it succeeds it will return the
// migrated block, otherwise it will return nil.
func decodeBlockBreakingChanges(encoded []byte) *Block {
	b0 := &BlockV0{}
	if err := gethRLP.DecodeBytes(encoded, b0); err == nil {
		return &Block{
			ParentBlockHash:     b0.ParentBlockHash,
			Height:              b0.Height,
			Timestamp:           b0.Timestamp,
			TotalSupply:         b0.TotalSupply,
			ReceiptRoot:         b0.ReceiptRoot,
			TransactionHashRoot: b0.TransactionHashRoot,
			TotalGasUsed:        b0.TotalGasUsed,
			Prevrandao:          gethCommon.Hash{},
		}
	}
	return nil
}

// decodeBlockProposalBreakingChanges will try to decode the bytes into all
// previous versions of block proposal type, if it succeeds it will return the
// migrated block, otherwise it will return nil.
func decodeBlockProposalBreakingChanges(encoded []byte) *BlockProposal {
	bp0 := &BlockProposalV0{}
	if err := gethRLP.DecodeBytes(encoded, bp0); err == nil {
		return &BlockProposal{
			Block: Block{
				ParentBlockHash:     bp0.ParentBlockHash,
				Height:              bp0.Height,
				Timestamp:           bp0.Timestamp,
				TotalSupply:         bp0.TotalSupply,
				ReceiptRoot:         bp0.ReceiptRoot,
				TransactionHashRoot: bp0.TransactionHashRoot,
				TotalGasUsed:        bp0.TotalGasUsed,
				Prevrandao:          gethCommon.Hash{},
			},
			Receipts: bp0.Receipts,
			TxHashes: bp0.TxHashes,
		}
	}
	return nil
}
