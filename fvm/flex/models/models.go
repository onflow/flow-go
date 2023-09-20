package models

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/onflow/cadence"
)

// Flex is an account inside FVM with special access to the underlying infrasturcture which
// allows to run a virtual evm-based blockchain on top of FVM. There are two ways to interact with this
// blockchain (flex env), first by passing signed transactions (EOA accounts) through Flex.Run method and each transaction
// would be considered as a block, updating the internal merkle tree of the Flex env and emitting a new root hash.
// The second way is through a new form of accounts called (FOA) which acts as a resource that any account on Flow environment
// could own and it has a collision-proof allocated flex address and any one who owns the resource, would be able to
// interact with the Flex environment on behalf of the flex address.

// The Flex enviornment shares the same native token as Flow, there is no new tokens minted and tokens could be bridged between
// FOA resources and Flow accounts. Then it could be circulated to any address space in the Flex (EOA, smart contracts), etc.

// Flex addresses are evm-compatible addresses
type FlexAddress common.Address

func NewFlexAddressFromString(str string) FlexAddress {
	return FlexAddress(common.BytesToAddress([]byte(str)))
}

// FlexBlock captures block info such as height and state
type FlexBlock interface {
	// returns the height of this block (autoincrement number)
	Height() uint64
	// returns the root hash of the state after executing this block
	StateRoot() common.Hash
	// returns the root hash of the events emited during execution of this block
	EventRoot() common.Hash
}

// Balance of a flex account
// a separate type has been considered here to prevent
// accidental dev mistakes when dealing with the conversion
type Balance interface {
	InAttoFlow() uint64
	InFlow() cadence.UFix64
}

type Gaslimit uint64

type Code []byte

type Data []byte

type FlexAccount interface {
	Address() FlexAddress
	Exists() bool
	Balance() Balance
}

type FlowTokenVault interface {
	Balance() Balance
	Withdraw(Balance) FlowTokenVault
	Deposit(FlowTokenVault)
}

// FlowOwnedAccount is a new type of accounts on the Flex environment
// that instead of being managed by public key inside the Flex, it would be managed
// as a resource inside the FVM accounts. in other words, the FVM account who holds
// a owns the FOA resource, could bridge native token from and to flex account associated with the FOA
// and deploys contracts or calls method on contracts without the need to sign a transaction.
type FlowOwnedAccount interface {
	// Address returns the flex address associated with the FOA account
	Address() *FlexAddress

	// Deposit deposits the token from the given vault into the Flex main vault
	// and update the FOA balance with the new amount
	Deposit(FlowTokenVault)

	// Withdraw deducts the balance from the FOA account and
	// withdraw and return flow token from the Flex main vault.
	Withdraw(Balance) FlowTokenVault

	// Deploy deploys a contract to the Flex environment
	// the new deployed contract would be at the returned address and
	// the contract data is not controlled by the FOA accounts
	Deploy(Code, Gaslimit, Balance) FlexAddress

	// Call calls a smart contract function with the given data
	// it would limit the gas used according to the limit provided
	// given it doesn't goes beyond what Flow transaction allows.
	// the balance would be deducted from the OFA account and would be transferred to the target address
	// contract data is not controlled by the FOA accounts
	Call(FlexAddress, Data, Gaslimit, Balance) Data
}

// Flex contract handles operations on the flex environment
type FlexContractHandler interface {
	// constructs a new flow owned account
	NewFlowOwnedAccount() FlowOwnedAccount

	// returns the last executed block info
	LastExecutedBlock() FlexBlock

	// runs a transaction in the Flex environment and collect
	// the flex gas fees under coinbase account, if tx is success full it returns true
	Run(bytes []byte, coinbase FlexAddress) bool
}
