package types

import (
	"fmt"
	"math/big"

	gethCommon "github.com/ethereum/go-ethereum/common"
	gethCore "github.com/ethereum/go-ethereum/core"
	gethTypes "github.com/ethereum/go-ethereum/core/types"
	gethParams "github.com/ethereum/go-ethereum/params"
	"github.com/ethereum/go-ethereum/rlp"
)

const (
	// tx type 255 is used for direct calls from COAs
	DirectCallTxType = byte(255)

	UnknownCallSubType  = byte(0)
	DepositCallSubType  = byte(1)
	WithdrawCallSubType = byte(2)
	TransferCallSubType = byte(3)
	DeployCallSubType   = byte(4)
	ContractCallSubType = byte(5)

	// Note that these gas values might need to change if we
	// change the transaction (e.g. add accesslist),
	// then it has to be updated to use Intrinsic function
	// to calculate the minimum gas needed to run the transaction.
	IntrinsicFeeForTokenTransfer = gethParams.TxGas

	// 21_000 is the minimum for a transaction + max gas allowed for receive/fallback methods
	DefaultGasLimitForTokenTransfer = IntrinsicFeeForTokenTransfer + 2_300

	// the value is set to the gas limit for transfer to facilitate transfers
	// to smart contract addresses.
	DepositCallGasLimit  = DefaultGasLimitForTokenTransfer
	WithdrawCallGasLimit = DefaultGasLimitForTokenTransfer
)

// DirectCall captures all the data related to a direct call to evm
// direct calls are similar to transactions but they don't have
// signatures and don't need sequence number checks
// Note that eventhough we don't check the nonce, it impacts
// hash calculation and also impacts the address of resulting contract
// when deployed through direct calls.
// Users don't have the worry about the nonce, and the handler sets
// it to the right value.
type DirectCall struct {
	Type     byte
	SubType  byte
	From     Address
	To       Address
	Data     []byte
	Value    *big.Int
	GasLimit uint64
	Nonce    uint64
}

// DirectCallFromEncoded constructs a DirectCall from encoded data
func DirectCallFromEncoded(encoded []byte) (*DirectCall, error) {
	if encoded[0] != DirectCallTxType {
		return nil, fmt.Errorf("tx type mismatch")
	}
	dc := &DirectCall{}
	return dc, rlp.DecodeBytes(encoded[1:], dc)
}

// Encode encodes the direct call it also adds the type
// as the very first byte, similar to how evm encodes types.
func (dc *DirectCall) Encode() ([]byte, error) {
	encoded, err := rlp.EncodeToBytes(dc)
	return append([]byte{dc.Type}, encoded...), err
}

// Hash computes the hash of a direct call
func (dc *DirectCall) Hash() (gethCommon.Hash, error) {
	// we use geth transaction hash calculation since direct call hash is included in the
	// block transaction hashes, and thus observed as any other transaction
	return dc.Transaction().Hash(), nil
}

// Message constructs a core.Message from the direct call
func (dc *DirectCall) Message() *gethCore.Message {
	return &gethCore.Message{
		From:      dc.From.ToCommon(),
		To:        dc.to(),
		Value:     dc.Value,
		Data:      dc.Data,
		Nonce:     dc.Nonce,
		GasLimit:  dc.GasLimit,
		GasPrice:  big.NewInt(0), // price is set to zero fo direct calls
		GasTipCap: big.NewInt(0), // also known as maxPriorityFeePerGas (in GWei)
		GasFeeCap: big.NewInt(0), // also known as maxFeePerGas (in GWei)
		// AccessList:        tx.AccessList(), // TODO revisit this value, the cost matter but performance might
		SkipAccountChecks: true, // this would let us not set the nonce
	}
}

// Transaction constructs a geth.Transaction from the direct call
func (dc *DirectCall) Transaction() *gethTypes.Transaction {
	return gethTypes.NewTx(&gethTypes.LegacyTx{
		GasPrice: big.NewInt(0),
		Gas:      dc.GasLimit,
		To:       dc.to(),
		Value:    dc.Value,
		Data:     dc.Data,
		Nonce:    dc.Nonce,
	})
}

// EmptyToField returns true if `to` field contains an empty address
func (dc *DirectCall) EmptyToField() bool {
	return dc.To == EmptyAddress
}

func (dc *DirectCall) to() *gethCommon.Address {
	var to *gethCommon.Address
	if !dc.EmptyToField() {
		ct := dc.To.ToCommon()
		to = &ct
	}
	return to
}

func NewDepositCall(
	bridge Address,
	address Address,
	amount *big.Int,
	nonce uint64,
) *DirectCall {
	return &DirectCall{
		Type:     DirectCallTxType,
		SubType:  DepositCallSubType,
		From:     bridge,
		To:       address,
		Data:     nil,
		Value:    amount,
		GasLimit: DepositCallGasLimit,
		Nonce:    nonce,
	}
}

func NewWithdrawCall(
	bridge Address,
	address Address,
	amount *big.Int,
	nonce uint64,
) *DirectCall {
	return &DirectCall{
		Type:     DirectCallTxType,
		SubType:  WithdrawCallSubType,
		From:     address,
		To:       bridge,
		Data:     nil,
		Value:    amount,
		GasLimit: WithdrawCallGasLimit,
		Nonce:    nonce,
	}
}

func NewTransferCall(
	from Address,
	to Address,
	amount *big.Int,
	nonce uint64,
) *DirectCall {
	return &DirectCall{
		Type:     DirectCallTxType,
		SubType:  TransferCallSubType,
		From:     from,
		To:       to,
		Data:     nil,
		Value:    amount,
		GasLimit: DefaultGasLimitForTokenTransfer,
		Nonce:    nonce,
	}
}

func NewDeployCall(
	caller Address,
	code Code,
	gasLimit uint64,
	value *big.Int,
	nonce uint64,
) *DirectCall {
	return &DirectCall{
		Type:     DirectCallTxType,
		SubType:  DeployCallSubType,
		From:     caller,
		To:       EmptyAddress,
		Data:     code,
		Value:    value,
		GasLimit: gasLimit,
		Nonce:    nonce,
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
	nonce uint64,
) *DirectCall {
	return &DirectCall{
		Type:     DirectCallTxType,
		SubType:  DeployCallSubType,
		From:     caller,
		To:       to,
		Data:     code,
		Value:    value,
		GasLimit: gasLimit,
		Nonce:    nonce,
	}
}

func NewContractCall(
	caller Address,
	to Address,
	data Data,
	gasLimit uint64,
	value *big.Int,
	nonce uint64,
) *DirectCall {
	return &DirectCall{
		Type:     DirectCallTxType,
		SubType:  ContractCallSubType,
		From:     caller,
		To:       to,
		Data:     data,
		Value:    value,
		GasLimit: gasLimit,
		Nonce:    nonce,
	}
}

type GasLimit uint64

type Code []byte

type Data []byte

// AsBigInt process the data and return it as a big integer
func (d Data) AsBigInt() *big.Int {
	return new(big.Int).SetBytes(d)
}
