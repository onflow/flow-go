package flow

import (
	"fmt"

	"github.com/onflow/flow-go/utils/slices"
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

// getChainCodeWord derives the network type used for address generation from the globally
// configured chain ID.
func (c ChainID) getChainCodeWord() uint64 {
	switch c {
	case Mainnet:
		return 0
	case Testnet:
		return invalidCodeTestnet
	case Emulator:
		return invalidCodeEmulator
	default:
		panic(fmt.Sprintf("chain ID [%s] is invalid or does not support linear code address generation", c))
	}
}

type chainImpl interface {
	newAddressGeneratorAtIndex(index uint64) AddressGenerator
	// IsValid returns true if a given address is a valid account address on a given chain,
	// and false otherwise.
	//
	// This is an off-chain check that only tells whether the address format is
	// valid. If the function returns true, this does not mean
	// a Flow account with this address has been generated. Such a test would
	// require an on-chain check.
	// zeroAddress() fails the check. Although it has a valid format, no account
	// in Flow is assigned to zeroAddress().
	IsValid(address Address) bool
	chain() ChainID
}

// monotonicImpl is a simple implementation of adress generation
// where addresses are simply the index of the account.
type monotonicImpl struct{}

func (m *monotonicImpl) newAddressGeneratorAtIndex(index uint64) AddressGenerator {
	return &MonotonicAddressGenerator{
		index: index,
	}
}

// IsValid checks the validity of an address
func (m *monotonicImpl) IsValid(address Address) bool {
	return address.uint64() > 0 && address.uint64() <= maxIndex
}

func (m *monotonicImpl) chain() ChainID {
	return MonotonicEmulator
}

// linearCodeImpl is an implementation of the address generation
// using linear codes.
type linearCodeImpl struct {
	chainID ChainID
}

func (l *linearCodeImpl) newAddressGeneratorAtIndex(index uint64) AddressGenerator {
	return &linearCodeAddressGenerator{
		index:         index,
		chainCodeWord: l.chainID.getChainCodeWord(),
	}
}

// IsValid checks the validity of an address
func (l *linearCodeImpl) IsValid(address Address) bool {
	codeWord := address.uint64()
	codeWord ^= uint64(l.chainID.getChainCodeWord())

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

func (l *linearCodeImpl) chain() ChainID {
	return l.chainID
}

type addressedChain struct {
	chainImpl
}

var mainnet = &addressedChain{
	chainImpl: &linearCodeImpl{
		chainID: Mainnet,
	},
}

var testnet = &addressedChain{
	chainImpl: &linearCodeImpl{
		chainID: Testnet,
	},
}

var emulator = &addressedChain{
	chainImpl: &linearCodeImpl{
		chainID: Emulator,
	},
}

var monotonicEmulator = &addressedChain{
	chainImpl: &monotonicImpl{},
}

// Chain returns the Chain corresponding to the string input
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

// Chain is the interface for address generation implementations.
type Chain interface {
	NewAddressGenerator() AddressGenerator
	AddressAtIndex(index uint64) (Address, error)
	ServiceAddress() Address
	BytesToAddressGenerator(b []byte) AddressGenerator
	IsValid(Address) bool
	String() string
	// required for tests
	zeroAddress() Address
	newAddressGeneratorAtIndex(index uint64) AddressGenerator
}

// NewAddressGenerator returns a new AddressGenerator with an
// initialized index.
func (id *addressedChain) NewAddressGenerator() AddressGenerator {
	return id.newAddressGeneratorAtIndex(0)
}

// AddressAtIndex returns the index-th generated account address.
func (id *addressedChain) AddressAtIndex(index uint64) (Address, error) {
	if index > maxIndex {
		return EmptyAddress, fmt.Errorf("index must be less or equal to %x", maxIndex)
	}
	return id.newAddressGeneratorAtIndex(index).CurrentAddress(), nil
}

// ServiceAddress returns the root (first) generated account address.
func (id *addressedChain) ServiceAddress() Address {
	// returned error is guaranteed to be nil
	address, _ := id.AddressAtIndex(1)
	return address
}

// zeroAddress returns the "zero address" (account that no one owns).
func (id *addressedChain) zeroAddress() Address {
	// returned error is guaranteed to be nil
	address, _ := id.AddressAtIndex(0)
	return address
}

// BytesToAddressGenerator converts an array of bytes into an address index
func (id *addressedChain) BytesToAddressGenerator(b []byte) AddressGenerator {
	bytes := slices.EnsureByteSliceSize(b, addressIndexLength)

	index := uint48(bytes[:])
	return id.newAddressGeneratorAtIndex(index)
}

func (id *addressedChain) String() string {
	return string(id.chain())
}
