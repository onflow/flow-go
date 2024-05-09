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
func (b *Block) CalculateGasUsage(results []Result) {
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
		res = decodeBlockBreakingChanges(encoded)
		if res == nil {
			return nil, err
		}
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

// Below block type section, defines earlier block types,
// this is being used to decode blocks that were stored
// before block type changes. It allows us to still decode
// a block that would otherwise be invalid if decoded into
// latest version of the above Block type.

type blockV0 struct {
	ParentBlockHash gethCommon.Hash
	Height          uint64
	UUIDIndex       uint64
	TotalSupply     uint64
	StateRoot       gethCommon.Hash
	ReceiptRoot     gethCommon.Hash
}

// adds TransactionHashes

type blockV1 struct {
	ParentBlockHash   gethCommon.Hash
	Height            uint64
	UUIDIndex         uint64
	TotalSupply       uint64
	StateRoot         gethCommon.Hash
	ReceiptRoot       gethCommon.Hash
	TransactionHashes []gethCommon.Hash
}

// removes UUIDIndex

type blockV2 struct {
	ParentBlockHash   gethCommon.Hash
	Height            uint64
	TotalSupply       uint64
	StateRoot         gethCommon.Hash
	ReceiptRoot       gethCommon.Hash
	TransactionHashes []gethCommon.Hash
}

// removes state root

type blockV3 struct {
	ParentBlockHash   gethCommon.Hash
	Height            uint64
	TotalSupply       uint64
	ReceiptRoot       gethCommon.Hash
	TransactionHashes []gethCommon.Hash
}

// change total supply type

type blockV4 struct {
	ParentBlockHash   gethCommon.Hash
	Height            uint64
	TotalSupply       *big.Int
	ReceiptRoot       gethCommon.Hash
	TransactionHashes []gethCommon.Hash
}

// adds timestamp

type blockV5 struct {
	ParentBlockHash   gethCommon.Hash
	Height            uint64
	Timestamp         uint64
	TotalSupply       *big.Int
	ReceiptRoot       gethCommon.Hash
	TransactionHashes []gethCommon.Hash
}

// decodeBlockBreakingChanges will try to decode the bytes into all
// previous versions of block type, if it succeeds it will return the
// migrated block, otherwise it will return nil.
func decodeBlockBreakingChanges(encoded []byte) *Block {
	b0 := &blockV0{}
	if err := gethRLP.DecodeBytes(encoded, b0); err == nil {
		return &Block{
			ParentBlockHash: b0.ParentBlockHash,
			Height:          b0.Height,
			ReceiptRoot:     b0.ReceiptRoot,
			TotalSupply:     big.NewInt(int64(b0.TotalSupply)),
		}
	}

	b1 := &blockV1{}
	if err := gethRLP.DecodeBytes(encoded, b1); err == nil {
		return &Block{
			ParentBlockHash:   b1.ParentBlockHash,
			Height:            b1.Height,
			TotalSupply:       big.NewInt(int64(b1.TotalSupply)),
			ReceiptRoot:       b1.ReceiptRoot,
			TransactionHashes: b1.TransactionHashes,
		}
	}

	b2 := &blockV2{}
	if err := gethRLP.DecodeBytes(encoded, b2); err == nil {
		return &Block{
			ParentBlockHash:   b2.ParentBlockHash,
			Height:            b2.Height,
			TotalSupply:       big.NewInt(int64(b2.TotalSupply)),
			ReceiptRoot:       b2.ReceiptRoot,
			TransactionHashes: b2.TransactionHashes,
		}
	}

	b3 := &blockV3{}
	if err := gethRLP.DecodeBytes(encoded, b3); err == nil {
		return &Block{
			ParentBlockHash:   b3.ParentBlockHash,
			Height:            b3.Height,
			TotalSupply:       big.NewInt(int64(b3.TotalSupply)),
			ReceiptRoot:       b3.ReceiptRoot,
			TransactionHashes: b3.TransactionHashes,
		}
	}

	b4 := &blockV4{}
	if err := gethRLP.DecodeBytes(encoded, b4); err == nil {
		return &Block{
			ParentBlockHash:   b4.ParentBlockHash,
			Height:            b4.Height,
			TotalSupply:       b4.TotalSupply,
			ReceiptRoot:       b4.ReceiptRoot,
			TransactionHashes: b4.TransactionHashes,
		}
	}

	b5 := &blockV5{}
	if err := gethRLP.DecodeBytes(encoded, b5); err == nil {
		return &Block{
			ParentBlockHash:   b5.ParentBlockHash,
			Height:            b5.Height,
			Timestamp:         b5.Timestamp,
			TotalSupply:       b5.TotalSupply,
			ReceiptRoot:       b5.ReceiptRoot,
			TransactionHashes: b5.TransactionHashes,
		}
	}

	return nil
}
