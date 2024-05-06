package types

import (
	"math/big"

	gethCommon "github.com/onflow/go-ethereum/common"
	gethTypes "github.com/onflow/go-ethereum/core/types"
	gethCrypto "github.com/onflow/go-ethereum/crypto"
	gethRLP "github.com/onflow/go-ethereum/rlp"
	gethTrie "github.com/onflow/go-ethereum/trie"
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

	// transaction hashes
	TransactionHashes []gethCommon.Hash

	// stores gas used by all transactions included in the block.
	TotalGasUsed uint64
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

// PopulateReceiptRoot populates receipt root with the given results
func (b *Block) PopulateReceiptRoot(results []*Result) {
	if len(results) == 0 {
		b.ReceiptRoot = gethTypes.EmptyReceiptsHash
		return
	}

	receipts := make(gethTypes.Receipts, 0)
	for _, res := range results {
		r := res.Receipt()
		if r == nil {
			continue
		}
		receipts = append(receipts, r)
	}
	b.ReceiptRoot = gethTypes.DeriveSha(receipts, gethTrie.NewStackTrie(nil))
}

// CalculateGasUsage sums up all the gas transactions in the block used
func (b *Block) CalculateGasUsage(results []*Result) {
	for _, res := range results {
		b.TotalGasUsed += res.GasConsumed
	}
}

// AppendTxHash appends a transaction hash to the list of transaction hashes of the block
func (b *Block) AppendTxHash(txHash gethCommon.Hash) {
	b.TransactionHashes = append(b.TransactionHashes, txHash)
}

// NewBlock constructs a new block
func NewBlock(
	parentBlockHash gethCommon.Hash,
	height uint64,
	timestamp uint64,
	totalSupply *big.Int,
	receiptRoot gethCommon.Hash,
	txHashes []gethCommon.Hash,
) *Block {
	return &Block{
		ParentBlockHash:   parentBlockHash,
		Height:            height,
		Timestamp:         timestamp,
		TotalSupply:       totalSupply,
		ReceiptRoot:       receiptRoot,
		TransactionHashes: txHashes,
	}
}

// NewBlockFromBytes constructs a new block from encoded data
func NewBlockFromBytes(encoded []byte) (*Block, error) {
	res := &Block{}

	err := gethRLP.DecodeBytes(encoded, res)
	if err != nil {
		// if decoding fails, try to decode to previous block type (without timestamp)
		r := struct {
			ParentBlockHash   gethCommon.Hash
			Height            uint64
			TotalSupply       *big.Int
			ReceiptRoot       gethCommon.Hash
			TransactionHashes []gethCommon.Hash
			TotalGasUsed      uint64
		}{}
		if e := gethRLP.DecodeBytes(encoded, &r); e != nil {
			// if both error out, return first error since it's more relevant
			return nil, err
		}

		return &Block{
			ParentBlockHash:   r.ParentBlockHash,
			Height:            r.Height,
			TotalSupply:       r.TotalSupply,
			ReceiptRoot:       r.ReceiptRoot,
			TransactionHashes: r.TransactionHashes,
			TotalGasUsed:      r.TotalGasUsed,
		}, nil
	}

	return res, nil
}

// GenesisBlock is the genesis block in the EVM environment
var GenesisBlock = &Block{
	ParentBlockHash: gethCommon.Hash{},
	Height:          uint64(0),
	TotalSupply:     new(big.Int),
	ReceiptRoot:     gethTypes.EmptyRootHash,
}

var GenesisBlockHash, _ = GenesisBlock.Hash()
