package types

// Account is an EVM account, currently
// three types of accounts are supported on Flow EVM,
// externally owned accounts (EOAs), smart contract accounts and bridged accounts
// BridgedAccount is a new type of account in the environment,
// that instead of being managed by public key,
// it is managed by a resource owned by a Flow account.
//
// In other words, the FVM account who owns the FOA resource
// can bridge native tokens to and from the account associated with the bridged account,
// deploy contracts to the environment,
// or call methods on contracts without the need to sign a transaction.
type Account interface {
	// Returns the address of this account
	Address() Address

	// Returns balance of this account
	Balance() Balance

	// Deposit deposits the token from the given vault into this account
	Deposit(*FLOWTokenVault)

	// Withdraw withdraws the balance from account and
	// return it as a FlowTokenVault
	// works only for bridged accounts
	Withdraw(Balance) *FLOWTokenVault

	// Transfer is a utility method on top of call for transfering tokens to another account
	Transfer(to Address, balance Balance)

	// Deploy deploys a contract to the environment
	// the new deployed contract would be at the returned address and
	// the contract data is not controlled by the bridge account
	// works only for bridged accounts
	Deploy(Code, GasLimit, Balance) Address

	// Call calls a smart contract function with the given data.
	// The gas usage is limited by the given gas limit,
	// and the Flow transaction's computation limit.
	// The fees are deducted from the bridged account
	// and are transferred to the target address.
	// if no data is provided it would behave as transfering tokens to the
	// target address
	// works only for bridged accounts
	Call(Address, Data, GasLimit, Balance) Data
}
