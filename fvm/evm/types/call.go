package types

import (
	"math/big"

	gethCommon "github.com/ethereum/go-ethereum/common"
	gethCore "github.com/ethereum/go-ethereum/core"
	gethCrypto "github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/rlp"
)

const (
	// tx type 255 is used for direct calls from bridged accounts
	DirectCallTxType = byte(255)

	UnknownCallSubType  = byte(0)
	DepositCallSubType  = byte(1)
	WithdrawCallSubType = byte(2)
	TransferCallSubType = byte(3)
	DeployCallSubType   = byte(4)
	ContractCallSubType = byte(5)

	TransferGasUsage = 21_000
)

// DirectCall captures all the data related to a direct call to evm
// direct calls are similar to transactions but they don't have
// signatures and don't need sequence number checks
type DirectCall struct {
	Type     byte
	SubType  byte
	From     Address
	To       Address
	Data     []byte
	Value    *big.Int
	GasLimit uint64
}

// Encode encodes the direct call it also adds the type
// as the very first byte, similar to how evm encodes types.
func (dc *DirectCall) Encode() ([]byte, error) {
	encoded, err := rlp.EncodeToBytes(dc)
	return append([]byte{dc.Type}, encoded...), err
}

// Hash computes the hash of a direct call
func (dc *DirectCall) Hash() (gethCommon.Hash, error) {
	encoded, err := dc.Encode()
	return gethCrypto.Keccak256Hash(encoded), err
}

// Message constructs a core.Message from the direct call
func (dc *DirectCall) Message() *gethCore.Message {
	var to *gethCommon.Address
	if dc.HasNonEmptyToField() {
		ct := dc.To.ToCommon()
		to = &ct
	}
	return &gethCore.Message{
		From:      dc.From.ToCommon(),
		To:        to,
		Value:     dc.Value,
		Data:      dc.Data,
		GasLimit:  dc.GasLimit,
		GasPrice:  big.NewInt(0), // price is set to zero fo direct calls
		GasTipCap: big.NewInt(1), // also known as maxPriorityFeePerGas
		GasFeeCap: big.NewInt(2), // also known as maxFeePerGas
		// AccessList:        tx.AccessList(), // TODO revisit this value, the cost matter but performance might
		SkipAccountChecks: true, // this would let us not set the nonce
	}
}

// HasNonEmptyTo returns true if to has a non empty value
func (dc *DirectCall) HasNonEmptyToField() bool {
	return dc.To != EmptyAddress
}

func NewDepositCall(address Address, amount *big.Int) *DirectCall {
	return &DirectCall{
		Type:     DirectCallTxType,
		SubType:  DepositCallSubType,
		From:     EmptyAddress,
		To:       address,
		Data:     nil,
		Value:    amount,
		GasLimit: TransferGasUsage,
	}
}

func NewWithdrawCall(address Address, amount *big.Int) *DirectCall {
	return &DirectCall{
		Type:     DirectCallTxType,
		SubType:  WithdrawCallSubType,
		From:     address,
		To:       EmptyAddress,
		Data:     nil,
		Value:    amount,
		GasLimit: TransferGasUsage,
	}
}

func NewTransferCall(from Address, to Address, amount *big.Int) *DirectCall {
	return &DirectCall{
		Type:     DirectCallTxType,
		SubType:  TransferCallSubType,
		From:     from,
		To:       to,
		Data:     nil,
		Value:    amount,
		GasLimit: TransferGasUsage,
	}
}

func NewDeployCall(
	caller Address,
	code Code,
	gasLimit uint64,
	value *big.Int,
) *DirectCall {
	return &DirectCall{
		Type:     DirectCallTxType,
		SubType:  DeployCallSubType,
		From:     caller,
		To:       EmptyAddress,
		Data:     code,
		Value:    value,
		GasLimit: gasLimit,
	}
}

// this subtype should only be used internally for
// deploying contracts at given addresses (e.g. COA account init setup)
// should not be used for other means.
func NewDeployCallWithTargetAddress(
	caller Address,
	to Address,
	code Code,
	gasLimit uint64,
	value *big.Int,
) *DirectCall {
	return &DirectCall{
		Type:     DirectCallTxType,
		SubType:  DeployCallSubType,
		From:     caller,
		To:       to,
		Data:     code,
		Value:    value,
		GasLimit: gasLimit,
	}
}

func NewContractCall(
	caller Address,
	to Address,
	data Data,
	gasLimit uint64,
	value *big.Int,
) *DirectCall {
	return &DirectCall{
		Type:     DirectCallTxType,
		SubType:  ContractCallSubType,
		From:     caller,
		To:       to,
		Data:     data,
		Value:    value,
		GasLimit: gasLimit,
	}
}
