package flow

import (
	"fmt"

	"github.com/dapperlabs/flow-go/utils/slices"
)

// A ChainID is a unique identifier for a specific Flow network instance.
//
// Chain IDs are used used to prevent replay attacks and to support network-specific address generation.
type ChainID string

// Mainnet is the chain ID for the mainnet node chain.
const Mainnet ChainID = "flow-mainnet"

// Testnet is the chain ID for the testnet node chain.
const Testnet ChainID = "flow-testnet"

// Emulator is the chain ID for the emulated node chain.
const Emulator ChainID = "flow-emulator"

// MonotonicEmulator is the chain ID for the emulated node chain with monotonic address generation.
const MonotonicEmulator ChainID = "flow-emulator-monotonic"

// networkType is the type of network for which account addresses
// are generated and checked.
//
// A valid address in one network is invalid in the other networks.
type networkType uint64

// getNetworkType derives the network type used for address generation from the globally
// configured chain ID.
func (c ChainID) getNetworkType() networkType {
	switch c {
	case Mainnet:
		return networkType(0)
	case Testnet:
		return networkType(invalidCodeTestnet)
	case Emulator:
		return networkType(invalidCodeEmulator)
	default:
		panic(fmt.Sprintf("chain ID [%s] is invalid or does not support linear code address generation", c))
	}
}

type chainImpl interface {
	newAddressGeneratorAtState(state uint64) AddressGenerator
	isValid(address Address) bool
}

type MonotonicImpl struct{}

func (m *MonotonicImpl) newAddressGeneratorAtState(state uint64) AddressGenerator {
	return &MonotonicAddressGenerator{
		state: state,
	}
}
func (m *MonotonicImpl) isValid(address Address) bool {
	return address.uint64() > 0 && address.uint64() < maxState
}

type LinearCodeImpl struct {
	chainID ChainID
}

func (l *LinearCodeImpl) newAddressGeneratorAtState(state uint64) AddressGenerator {
	return &LinearCodeAddressGenerator{
		state:   state,
		chainID: l.chainID,
	}
}
func (m *LinearCodeImpl) isValid(address Address) bool {
	codeWord := address.uint64()
	codeWord ^= uint64(m.chainID.getNetworkType())

	if codeWord == 0 {
		return false
	}

	// Multiply the code word GF(2)-vector by the parity-check matrix
	parity := uint(0)
	for i := 0; i < linearCodeN; i++ {
		if codeWord&1 == 1 {
			parity ^= parityCheckMatrixColumns[i]
		}
		codeWord >>= 1
	}
	return parity == 0
}

type addressedChain struct {
	impl    chainImpl
	chainID ChainID
}

var mainnet = &addressedChain{
	chainID: Mainnet,
	impl: &LinearCodeImpl{
		chainID: Mainnet,
	},
}

var testnet = &addressedChain{
	chainID: Testnet,
	impl: &LinearCodeImpl{
		chainID: Testnet,
	},
}

var emulator = &addressedChain{
	chainID: Emulator,
	impl: &LinearCodeImpl{
		chainID: Emulator,
	},
}

var monotonicEmulator = &addressedChain{
	chainID: Emulator,
	impl:    &MonotonicImpl{},
}

func (c ChainID) Chain() Chain {
	switch c {
	case Mainnet:
		return mainnet
	case Testnet:
		return testnet
	case Emulator:
		return emulator
	case MonotonicEmulator:
		return monotonicEmulator
	default:
		panic(fmt.Sprintf("chain ID [%s] is invalid ", c))
	}
}

func (c ChainID) String() string {
	return string(c)
}

type Chain interface {
	NewAddressGenerator() AddressGenerator
	AddressAtIndex(index uint64) (Address, error)
	ServiceAddress() Address
	ZeroAddress() Address
	BytesToAddressGenerator(b []byte) AddressGenerator
	IsValid(Address) bool
	newAddressGeneratorAtState(state uint64) AddressGenerator
	String() string
}

// NewAddressGenerator returns a new AddressGenerator with an
// initialized state.
func (id *addressedChain) NewAddressGenerator() AddressGenerator {
	return id.newAddressGeneratorAtState(0)
}

func (id *addressedChain) newAddressGeneratorAtState(state uint64) AddressGenerator {
	return id.impl.newAddressGeneratorAtState(state)
}

// AddressAtIndex returns the nth generated account address.
func (id *addressedChain) AddressAtIndex(index uint64) (Address, error) {
	if index >= maxState {
		return EmptyAddress, fmt.Errorf("index must be less than %x", maxState)
	}
	return id.newAddressGeneratorAtState(index).CurrentAddress(), nil
}

// ServiceAddress returns the root (first) generated account address.
func (id *addressedChain) ServiceAddress() Address {
	// returned error is guaranteed to be nil
	address, _ := id.AddressAtIndex(1)
	return address
}

// zeroAddress returns the "zero address" (account that no one owns).
func (id *addressedChain) ZeroAddress() Address {
	// returned error is guaranteed to be nil
	address, _ := id.AddressAtIndex(0)
	return address
}

// BytesToAddressGenerator converts an array of bytes into an address state
func (id *addressedChain) BytesToAddressGenerator(b []byte) AddressGenerator {
	bytes := slices.EnsureByteSliceSize(b, addressStateLength)

	state := uint48(bytes[:])
	return id.newAddressGeneratorAtState(state)
}

// IsValid returns true if a given address is a valid account address on given chain,
// and false otherwise.
//
// This is an off-chain check that only tells whether the address format is
// valid. If the function returns true, this does not mean
// a Flow account with this address has been generated. Such a test would
// require on on-chain check.
// zeroAddress() fails the check. Although it has a valid format, no account
// in Flow is assigned to zeroAddress().
func (id *addressedChain) IsValid(address Address) bool {
	return id.impl.isValid(address)
}

// IsValid returns true if a given address is a valid account address on given chain,
// and false otherwise.
//
// This is an off-chain check that only tells whether the address format is
// valid. If the function returns true, this does not mean
// a Flow account with this address has been generated. Such a test would
// require on on-chain check.
// zeroAddress() fails the check. Although it has a valid format, no account
// in Flow is assigned to zeroAddress().

func (id *addressedChain) String() string {
	return string(id.chainID)
}
