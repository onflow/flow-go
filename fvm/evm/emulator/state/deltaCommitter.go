package state

import (
	"encoding/binary"

	"github.com/holiman/uint256"
	"github.com/onflow/crypto/hash"
	gethCommon "github.com/onflow/go-ethereum/common"
)

type OpCode byte

const (
	UnknownOpCode OpCode = 0

	AccountCreationOpCode OpCode = 1
	AccountUpdateOpCode   OpCode = 2
	AccountDeletionOpCode OpCode = 3
	SlotUpdateOpCode      OpCode = 4
)

const (
	opcodeByteSize            = 1
	addressByteSize           = gethCommon.AddressLength
	nonceByteSize             = 8
	balanceByteSize           = 32
	hashByteSize              = gethCommon.HashLength
	accountDeletionBufferSize = opcodeByteSize + addressByteSize
	accountCreationBufferSize = opcodeByteSize +
		addressByteSize +
		nonceByteSize +
		balanceByteSize +
		hashByteSize
	accountUpdateBufferSize = accountCreationBufferSize
	slotUpdateBufferSize    = addressByteSize + hashByteSize + hashByteSize
)

// DeltaCommitter captures a
// commitment over deltas
type DeltaCommitter struct {
	hasher hash.Hasher
}

// NewDeltaCommitter constructs a new delta committer
func NewDeltaCommitter() *DeltaCommitter {
	return &DeltaCommitter{
		hasher: hash.NewSHA3_256(),
	}
}

func (dc *DeltaCommitter) CreateAccount(
	addr gethCommon.Address,
	balance *uint256.Int,
	nonce uint64,
	codeHash gethCommon.Hash,
) error {
	buffer := make([]byte, accountCreationBufferSize)
	var index int
	buffer[0] = byte(AccountCreationOpCode)
	index += opcodeByteSize
	copy(buffer[index:index+addressByteSize], addr.Bytes())
	index += addressByteSize
	copy(buffer[index:index+balanceByteSize], balance.Bytes())
	index += balanceByteSize
	binary.BigEndian.PutUint64(buffer[index:index+nonceByteSize], nonce)
	index += nonceByteSize
	copy(buffer[index:index+hashByteSize], codeHash.Bytes())
	_, err := dc.hasher.Write(buffer)
	return err
}

func (dc *DeltaCommitter) UpdateAccount(
	addr gethCommon.Address,
	balance *uint256.Int,
	nonce uint64,
	codeHash gethCommon.Hash,
) error {
	buffer := make([]byte, accountUpdateBufferSize)
	var index int
	buffer[0] = byte(AccountUpdateOpCode)
	index += opcodeByteSize
	copy(buffer[index:index+addressByteSize], addr.Bytes())
	index += addressByteSize
	copy(buffer[index:index+balanceByteSize], balance.Bytes())
	index += balanceByteSize
	binary.BigEndian.PutUint64(buffer[index:index+nonceByteSize], nonce)
	index += nonceByteSize
	copy(buffer[index:index+hashByteSize], codeHash.Bytes())
	_, err := dc.hasher.Write(buffer)
	return err
}

func (dc *DeltaCommitter) DeleteAccount(addr gethCommon.Address) error {
	buffer := make([]byte, accountDeletionBufferSize)
	var index int
	buffer[0] = byte(AccountDeletionOpCode)
	index += opcodeByteSize
	copy(buffer[index:index+addressByteSize], addr.Bytes())
	_, err := dc.hasher.Write(buffer)
	return err
}

func (dc *DeltaCommitter) UpdateSlot(
	addr gethCommon.Address,
	key gethCommon.Hash,
	value gethCommon.Hash,
) error {
	buffer := make([]byte, slotUpdateBufferSize)
	var index int
	buffer[0] = byte(SlotUpdateOpCode)
	index += opcodeByteSize
	copy(buffer[index:index+addressByteSize], addr.Bytes())
	index += addressByteSize
	copy(buffer[index:index+hashByteSize], key.Bytes())
	index += hashByteSize
	copy(buffer[index:index+hashByteSize], value.Bytes())
	index += hashByteSize
	_, err := dc.hasher.Write(buffer)
	return err
}

func (dc *DeltaCommitter) Commitment() hash.Hash {
	return dc.hasher.SumHash()
}
