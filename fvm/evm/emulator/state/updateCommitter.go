package state

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"

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
	slotUpdateBufferSize    = opcodeByteSize +
		addressByteSize +
		hashByteSize +
		hashByteSize
)

// UpdateCommitter captures operations (delta) through
// a set of calls (order matters) and constructs a commitment over the state changes.
type UpdateCommitter struct {
	hasher hash.Hasher
}

// NewUpdateCommitter constructs a new UpdateCommitter
func NewUpdateCommitter() *UpdateCommitter {
	return &UpdateCommitter{
		hasher: hash.NewSHA3_256(),
	}
}

// CreateAccount captures a create account operation
func (dc *UpdateCommitter) CreateAccount(
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
	encodedBalance := balance.Bytes32()
	copy(buffer[index:index+balanceByteSize], encodedBalance[:])
	index += balanceByteSize
	binary.BigEndian.PutUint64(buffer[index:index+nonceByteSize], nonce)
	index += nonceByteSize
	copy(buffer[index:index+hashByteSize], codeHash.Bytes())
	fmt.Println(hex.EncodeToString(buffer))
	_, err := dc.hasher.Write(buffer)
	return err
}

// CreateAccount captures an update account operation
func (dc *UpdateCommitter) UpdateAccount(
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
	encodedBalance := balance.Bytes32()
	copy(buffer[index:index+balanceByteSize], encodedBalance[:])
	index += balanceByteSize
	binary.BigEndian.PutUint64(buffer[index:index+nonceByteSize], nonce)
	index += nonceByteSize
	copy(buffer[index:index+hashByteSize], codeHash.Bytes())
	_, err := dc.hasher.Write(buffer)
	return err
}

// CreateAccount captures a delete account operation
func (dc *UpdateCommitter) DeleteAccount(addr gethCommon.Address) error {
	buffer := make([]byte, accountDeletionBufferSize)
	var index int
	buffer[0] = byte(AccountDeletionOpCode)
	index += opcodeByteSize
	copy(buffer[index:index+addressByteSize], addr.Bytes())
	_, err := dc.hasher.Write(buffer)
	return err
}

// CreateAccount captures a update slot operation
func (dc *UpdateCommitter) UpdateSlot(
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
	_, err := dc.hasher.Write(buffer)
	return err
}

// Commitment calculates and returns the commitment
func (dc *UpdateCommitter) Commitment() hash.Hash {
	return dc.hasher.SumHash()
}
