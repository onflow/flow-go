package types

import (
	"fmt"
	"math/big"

	gethCommon "github.com/onflow/go-ethereum/common"
	gethCore "github.com/onflow/go-ethereum/core"
	gethTypes "github.com/onflow/go-ethereum/core/types"
	gethParams "github.com/onflow/go-ethereum/params"
	"github.com/onflow/go-ethereum/rlp"
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
	// change the transaction (e.g. add access list),
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
// Note that while we don't check the nonce, it impacts
// hash calculation and also impacts the address of resulting contract
// when deployed through direct calls.
// Users don't have the worry about the nonce, handler sets
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
func (dc *DirectCall) Hash() gethCommon.Hash {
	// we use geth transaction hash calculation since direct call hash is included in the
	// block transaction hashes, and thus observed as any other transaction
	// We construct this Legacy tx type so the external 3rd party tools
	// don't have to support a new type for the purpose of hash computation
	return dc.Transaction().Hash()
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
		// TODO: maybe revisit setting the access list
		// AccessList:        tx.AccessList(),
		// When SkipAccountChecks is true, the message nonce is not checked against the
		// account nonce in state. It also disables checking that the sender is an EOA.
		// Since we use the direct calls for COAs, we set the nonce and the COA is an smart contract.
		SkipAccountChecks: true,
	}
}

// Transaction constructs a geth.Transaction from the direct call
func (dc *DirectCall) Transaction() *gethTypes.Transaction {
	// Since a direct call doesn't have a valid siganture
	// and we need to somehow include the From feild for the purpose
	// of hash calculation. we define the canonical format by
	// using the FROM bytes to set the bytes for the R part of the tx (big endian),
	// S captures the subtype of transaction and V is set to DirectCallTxType (255).
	return gethTypes.NewTx(&gethTypes.LegacyTx{
		GasPrice: big.NewInt(0),
		Gas:      dc.GasLimit,
		To:       dc.to(),
		Value:    dc.Value,
		Data:     dc.Data,
		Nonce:    dc.Nonce,
		R:        new(big.Int).SetBytes(dc.From.Bytes()),
		S:        new(big.Int).SetBytes([]byte{dc.SubType}),
		V:        new(big.Int).SetBytes([]byte{DirectCallTxType}),
	})
}

// EmptyToField returns true if `to` field contains an empty address
func (dc *DirectCall) EmptyToField() bool {
	return dc.To == EmptyAddress
}

func (dc *DirectCall) to() *gethCommon.Address {
	if !dc.EmptyToField() {
		ct := dc.To.ToCommon()
		return &ct
	}
	return nil
}

// NewDepositCall constructs a new deposit direct call
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

// NewDepositCall constructs a new withdraw direct call
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

// NewDeployCall constructs a new deploy direct call
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

// NewDeployCallWithTargetAddress constructs a new deployment call
// for the given target address
//
// Warning! This subtype should only be used internally for
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

// NewContractCall constructs a new contract call
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

// GasLimit sets the limit for the total gas used by a transaction
type GasLimit uint64

// Code holds an smart contract code
type Code []byte

// Data holds the data passed as part of a call
type Data []byte

// AsBigInt process the data and return it as a big integer
func (d Data) AsBigInt() *big.Int {
	return new(big.Int).SetBytes(d)
}
