package types

import (
	"bytes"
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

	// TransactionHashRoot returns the root hash of the transaction hashes
	// included in this block.
	// Note that despite similar functionality this is a bit different than TransactionRoot
	// provided by Ethereum. TransactionRoot constructs a Merkle proof with leafs holding
	// encoded transactions as values. But TransactionHashRoot uses transaction hash
	// values as node values. Proofs are still compatible but might require an extra hashing step.
	TransactionHashRoot gethCommon.Hash

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

// NewBlock constructs a new block
func NewBlock(
	parentBlockHash gethCommon.Hash,
	height uint64,
	timestamp uint64,
	totalSupply *big.Int,
) *Block {
	return &Block{
		ParentBlockHash: parentBlockHash,
		Height:          height,
		Timestamp:       timestamp,
		TotalSupply:     totalSupply,
		ReceiptRoot:     gethTypes.EmptyReceiptsHash,
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

// BlockProposal is a EVM block proposal
// holding all the iterim data of block before commitment
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
	if res == nil {
		return
	}
	b.TxHashes = append(b.TxHashes, res.TxHash)
	r := res.LightReceipt(b.TotalGasUsed)
	if r == nil {
		return
	}
	b.Receipts = append(b.Receipts, *r)
	b.TotalGasUsed = r.CumulativeGasUsed
}

// PopulateRoots populates receiptRoot and transactionHashRoot
func (b *BlockProposal) PopulateRoots() {
	if len(b.Receipts) == 0 {
		b.ReceiptRoot = gethTypes.EmptyReceiptsHash
		return
	}
	receipts := make(gethTypes.Receipts, len(b.Receipts))
	for i, lr := range b.Receipts {
		receipts[i] = lr.ToReceipt()
	}

	b.ReceiptRoot = gethTypes.DeriveSha(receipts, gethTrie.NewStackTrie(nil))

	// TODO: we can make this concurrent if its
	b.TransactionHashRoot = b.TxHashes.RootHash()
}

// ToBytes encodes the block proposal into bytes
func (b *BlockProposal) ToBytes() ([]byte, error) {
	return gethRLP.EncodeToBytes(b)
}

// NewBlockProposalFromBytes constructs a new block proposal from encoded data
func NewBlockProposalFromBytes(encoded []byte) (*BlockProposal, error) {
	res := &BlockProposal{}
	return res, gethRLP.DecodeBytes(encoded, res)
}

func NewBlockProposal(
	parentBlockHash gethCommon.Hash,
	height uint64,
	timestamp uint64,
	totalSupply *big.Int,
) *BlockProposal {
	return &BlockProposal{
		Block: Block{
			ParentBlockHash: parentBlockHash,
			Height:          height,
			Timestamp:       timestamp,
			TotalSupply:     totalSupply,
			ReceiptRoot:     gethTypes.EmptyRootHash,
		},
		Receipts: make([]LightReceipt, 0),
		TxHashes: make([]gethCommon.Hash, 0),
	}
}

type TransactionHashes []gethCommon.Hash

func (th TransactionHashes) Len() int {
	return len(th)
}

func (th TransactionHashes) EncodeIndex(index int, buffer *bytes.Buffer) {
	buffer.Write(th[index].Bytes())
}

func (txs TransactionHashes) RootHash() gethCommon.Hash {
	return gethTypes.DeriveSha(txs, gethTrie.NewStackTrie(nil))
}

// todo remove this if confirmed we no longer need it on testnet, mainnet and previewnet.

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

// adds total gas used

type blockV6 struct {
	ParentBlockHash   gethCommon.Hash
	Height            uint64
	Timestamp         uint64
	TotalSupply       *big.Int
	ReceiptRoot       gethCommon.Hash
	TransactionHashes []gethCommon.Hash
	TotalGasUsed      uint64
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
			ParentBlockHash: b1.ParentBlockHash,
			Height:          b1.Height,
			TotalSupply:     big.NewInt(int64(b1.TotalSupply)),
			ReceiptRoot:     b1.ReceiptRoot,
		}
	}

	b2 := &blockV2{}
	if err := gethRLP.DecodeBytes(encoded, b2); err == nil {
		return &Block{
			ParentBlockHash: b2.ParentBlockHash,
			Height:          b2.Height,
			TotalSupply:     big.NewInt(int64(b2.TotalSupply)),
			ReceiptRoot:     b2.ReceiptRoot,
		}
	}

	b3 := &blockV3{}
	if err := gethRLP.DecodeBytes(encoded, b3); err == nil {
		return &Block{
			ParentBlockHash: b3.ParentBlockHash,
			Height:          b3.Height,
			TotalSupply:     big.NewInt(int64(b3.TotalSupply)),
			ReceiptRoot:     b3.ReceiptRoot,
		}
	}

	b4 := &blockV4{}
	if err := gethRLP.DecodeBytes(encoded, b4); err == nil {
		return &Block{
			ParentBlockHash: b4.ParentBlockHash,
			Height:          b4.Height,
			TotalSupply:     b4.TotalSupply,
			ReceiptRoot:     b4.ReceiptRoot,
		}
	}

	b5 := &blockV5{}
	if err := gethRLP.DecodeBytes(encoded, b5); err == nil {
		return &Block{
			ParentBlockHash: b5.ParentBlockHash,
			Height:          b5.Height,
			Timestamp:       b5.Timestamp,
			TotalSupply:     b5.TotalSupply,
			ReceiptRoot:     b5.ReceiptRoot,
		}
	}

	b6 := &blockV6{}
	if err := gethRLP.DecodeBytes(encoded, b6); err == nil {
		return &Block{
			ParentBlockHash: b5.ParentBlockHash,
			Height:          b5.Height,
			Timestamp:       b5.Timestamp,
			TotalSupply:     b5.TotalSupply,
			ReceiptRoot:     b5.ReceiptRoot,
		}
	}

	return nil
}
