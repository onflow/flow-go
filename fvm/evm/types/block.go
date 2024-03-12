package types

import (
	"math/big"

	gethCommon "github.com/ethereum/go-ethereum/common"
	gethTypes "github.com/ethereum/go-ethereum/core/types"
	gethCrypto "github.com/ethereum/go-ethereum/crypto"
	gethRLP "github.com/ethereum/go-ethereum/rlp"
	gethTrie "github.com/onflow/go-ethereum/trie"
)

// Block represents a evm block.
// It captures block info such as height and state
type Block struct {
	// the hash of the parent block
	ParentBlockHash gethCommon.Hash

	// Height returns the height of this block
	Height uint64

	// holds the total amount of the native token deposited in the evm side. (in attoflow)
	TotalSupply *big.Int

	// ReceiptRoot returns the root hash of the receipts emitted in this block
	ReceiptRoot gethCommon.Hash

	// transaction hashes
	TransactionHashes []gethCommon.Hash
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

func (b *Block) PopulateReceiptRoot(results []Result) {
	if len(results) == 0 {
		b.ReceiptRoot = gethTypes.EmptyReceiptsHash
		return
	}

	receipts := make(gethTypes.Receipts, len(results))
	for i, res := range results {
		receipts[i] = res.Receipt()
	}
	b.ReceiptRoot = gethTypes.DeriveSha(receipts, gethTrie.NewStackTrie(nil))
}

// AppendTxHash appends a transaction hash to the list of transaction hashes of the block
func (b *Block) AppendTxHash(txHash gethCommon.Hash) {
	b.TransactionHashes = append(b.TransactionHashes, txHash)
}

// NewBlock constructs a new block
func NewBlock(
	parentBlockHash gethCommon.Hash,
	height uint64,
	totalSupply *big.Int,
	receiptRoot gethCommon.Hash,
	txHashes []gethCommon.Hash,
) *Block {
	return &Block{
		ParentBlockHash:   parentBlockHash,
		Height:            height,
		TotalSupply:       totalSupply,
		ReceiptRoot:       receiptRoot,
		TransactionHashes: txHashes,
	}
}

// NewBlockFromBytes constructs a new block from encoded data
func NewBlockFromBytes(encoded []byte) (*Block, error) {
	res := &Block{}
	err := gethRLP.DecodeBytes(encoded, res)
	return res, err
}

// GenesisBlock is the genesis block in the EVM environment
var GenesisBlock = &Block{
	ParentBlockHash: gethCommon.Hash{},
	Height:          uint64(0),
	TotalSupply:     new(big.Int),
	ReceiptRoot:     gethTypes.EmptyRootHash,
}

var GenesisBlockHash, _ = GenesisBlock.Hash()
