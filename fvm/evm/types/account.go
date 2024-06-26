package types

// Account is an EVM account, currently
// three types of accounts are supported on Flow EVM,
// externally owned accounts (EOAs), smart contract accounts and cadence owned accounts
// Cadence-owned-account (COA) is a new type of account in the environment,
// that instead of being managed by public key,
// it is managed by a resource owned by a Flow account.
//
// In other words, the FVM account who owns the FOA resource
// can bridge native tokens to and from the account associated with the COA,
// deploy contracts to the environment,
// or call methods on contracts without the need to sign a transaction.
type Account interface {
	// Address returns the address of this account
	Address() Address

	// Balance returns the balance of this account
	Balance() Balance

	// Code returns the code of this account
	Code() Code

	// CodeHash returns the code hash of this account
	CodeHash() []byte

	// Nonce returns the nonce of this account
	Nonce() uint64

	// Deposit deposits the token from the given vault into this account
	Deposit(*FLOWTokenVault)

	// Withdraw withdraws the balance from account and
	// return it as a FlowTokenVault
	// works only for COAs
	Withdraw(Balance) *FLOWTokenVault

	// Transfer is a utility method on top of call for transfering tokens to another account
	Transfer(to Address, balance Balance)

	// Deploy deploys a contract to the environment
	// the new deployed contract would be at the returned
	// result address and the contract data is not controlled by the COA
	// works only for COAs
	Deploy(Code, GasLimit, Balance) *ResultSummary

	// Call calls a smart contract function with the given data.
	// The gas usage is limited by the given gas limit,
	// and the Flow transaction's computation limit.
	// The fees are deducted from the COA
	// and are transferred to the target address.
	// if no data is provided it would behave as transfering tokens to the
	// target address
	// works only for COAs
	Call(Address, Data, GasLimit, Balance) *ResultSummary
}
