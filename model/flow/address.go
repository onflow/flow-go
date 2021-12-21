// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package flow

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"strings"
)

// Address represents the 8 byte address of an account.
type Address [AddressLength]byte

type AddressGenerator interface {
	NextAddress() (Address, error)
	CurrentAddress() Address
	Bytes() []byte
	AddressCount() uint64 // returns the total number of addresses that have been generated so far
}

type MonotonicAddressGenerator struct {
	index uint64
}

// linearCodeAddressGenerator represents the internal index of the linear code address generation mechanism
type linearCodeAddressGenerator struct {
	chainCodeWord uint64
	index         uint64
}

const (
	// AddressLength is the size of an account address in bytes.
	// (n) is the size of an account address in bits.
	AddressLength = (linearCodeN + 7) >> 3
	// addressIndexLength is the size of an account address state in bytes.
	// (k) is the size of an account address in bits.
	addressIndexLength = (linearCodeK + 7) >> 3
)

// EmptyAddress is the default value of a variable of type Address
var EmptyAddress = Address{}

// HexToAddress converts a hex string to an Address.
func HexToAddress(h string) Address {
	trimmed := strings.TrimPrefix(h, "0x")
	if len(trimmed)%2 == 1 {
		trimmed = "0" + trimmed
	}
	b, _ := hex.DecodeString(trimmed)
	return BytesToAddress(b)
}

// BytesToAddress returns Address with value b.
//
// If b is larger than 8, b will be cropped from the left.
// If b is smaller than 8, b will be appended by zeroes at the front.
func BytesToAddress(b []byte) Address {
	var a Address
	if len(b) > AddressLength {
		b = b[len(b)-AddressLength:]
	}
	copy(a[AddressLength-len(b):], b)
	return a
}

// Bytes returns the byte representation of the address.
func (a Address) Bytes() []byte {
	return a[:]
}

// Hex returns the hex string representation of the address.
func (a Address) Hex() string {
	return hex.EncodeToString(a.Bytes())
}

// HexWithPrefix returns the hex string representation of the address,
// including the 0x prefix.
func (a Address) HexWithPrefix() string {
	return "0x" + a.Hex()
}

// String returns the string representation of the address.
func (a Address) String() string {
	return a.Hex()
}

// Short returns the string representation of the address with leading zeros
// removed.
func (a Address) Short() string {
	hex := a.String()
	trimmed := strings.TrimLeft(hex, "0")
	if len(trimmed)%2 != 0 {
		trimmed = "0" + trimmed
	}
	return trimmed
}

func (a Address) MarshalJSON() ([]byte, error) {
	return []byte(fmt.Sprintf("\"%s\"", a.Hex())), nil
}

func (a *Address) UnmarshalJSON(data []byte) error {
	*a = HexToAddress(strings.Trim(string(data), "\""))
	return nil
}

// modified from binary.bigEndian.uint64
func uint48(b []byte) uint64 {
	_ = b[5] // bounds check hint to compiler;
	return uint64(b[5]) | uint64(b[4])<<8 |
		uint64(b[3])<<16 | uint64(b[2])<<24 | uint64(b[1])<<32 | uint64(b[0])<<40
}

// modified from binary.bigEndian.PutUint64
func putUint48(b []byte, v uint64) {
	_ = b[5] // early bounds check to guarantee safety of writes below
	b[0] = byte(v >> 40)
	b[1] = byte(v >> 32)
	b[2] = byte(v >> 24)
	b[3] = byte(v >> 16)
	b[4] = byte(v >> 8)
	b[5] = byte(v)
}

func indexToBytes(index uint64) []byte {
	indexBytes := make([]byte, addressIndexLength)
	putUint48(indexBytes, index)
	return indexBytes
}

// Bytes converts an address index into a slice of bytes
func (gen *MonotonicAddressGenerator) Bytes() []byte {
	return indexToBytes(gen.index)
}

// Bytes converts an address index into a slice of bytes
func (gen *linearCodeAddressGenerator) Bytes() []byte {
	return indexToBytes(gen.index)
}

// NextAddress increments the internal index and generates the new address
// corresponding to the new index.
func (gen *MonotonicAddressGenerator) NextAddress() (Address, error) {
	gen.index++
	if uint64(gen.index) > maxIndex {
		return EmptyAddress, fmt.Errorf("the new index value is not valid, it must be less or equal to %x", maxIndex)
	}
	return gen.CurrentAddress(), nil
}

// CurrentAddress returns the address corresponding to the internal index.
func (gen *MonotonicAddressGenerator) CurrentAddress() Address {
	return uint64ToAddress(gen.index)
}

// AddressCount returns the total number of addresses generated so far
func (gen *MonotonicAddressGenerator) AddressCount() uint64 {
	return gen.index
}

// NextAddress generates an account address from the addressing index.
//
// The address is generated for a specific network (Flow mainnet, testnet..)
// The second returned value is the new updated addressing index. The new
// addressing index should replace the old index to keep generating account
// addresses in a sequential way.
// Each index is mapped to exactly one address. There are as many addresses
// as indices.
// zeroAddress() corresponds to the index "0" while ServiceAddress() corresponds to the
// index "1".
func (gen *linearCodeAddressGenerator) NextAddress() (Address, error) {
	err := gen.nextIndex()
	if err != nil {
		return EmptyAddress, err
	}
	index := gen.index
	address := encodeWord(index)
	// customize the code word for a specific network
	address ^= gen.chainCodeWord
	return uint64ToAddress(address), nil
}

// CurrentAddress returns the current account address.
//
// The returned address is the address of the latest created account.
func (gen *linearCodeAddressGenerator) CurrentAddress() Address {
	index := gen.index
	address := encodeWord(index)
	// customize the code word for a specific network
	address ^= gen.chainCodeWord
	return uint64ToAddress(address)
}

// AddressCount returns the total number of addresses generated so far
func (gen *linearCodeAddressGenerator) AddressCount() uint64 {
	return gen.index
}

// increments the internal index of the generator.
//
// In this implemntation, the index values are simply
// incremented from 0 to 2^k-1.
func (gen *linearCodeAddressGenerator) nextIndex() error {
	gen.index++
	if uint64(gen.index) > maxIndex {
		return fmt.Errorf("the new index value is not valid, it must be less or equal to %x", maxIndex)
	}
	return nil
}

// uint64ToAddress returns an address with value v.
//
// The value v fits into the address as the address size is 8
func uint64ToAddress(v uint64) Address {
	var b [AddressLength]byte
	binary.BigEndian.PutUint64(b[:], v)
	return Address(b)
}

// uint64 converts an address into a uint64
func (a *Address) uint64() uint64 {
	v := binary.BigEndian.Uint64(a[:])
	return v
}

const (
	// [n,k,d]-Linear code parameters
	// The linear code used in the account addressing is a [64,45,7]
	// It generates a [64,45]-code, which is the space of Flow account addresses.
	//
	// n is the size of the code words in bits,
	// which is also the size of the account addresses in bits.
	linearCodeN = 64
	// k is size of the words in bits.
	// 2^k is the total number of possible account addresses.
	linearCodeK = 45
	// p is the number of code parity bits.
	// p = n - k
	//
	// d is the distance of the linear code.
	// It is the minimum hamming distance between any two Flow account addresses.
	// This means any pair of Flow addresses have at least 7 different bits, which
	// minimizes the mistakes of typing wrong addresses.
	// d is also the minimum hamming weight of all account addresses (the zero address is not an account address).
	linearCodeD = 7

	// the maximum value of the internal state, 2^k - 1.
	maxIndex = (1 << linearCodeK) - 1
)

// The following are invalid code-words in the [64,45] code.
// These constants are used to generate non-Flow-Mainnet addresses

// invalidCodeTestNetwork is the invalid codeword used for long-lived test networks.
const invalidCodeTestNetwork = uint64(0x6834ba37b3980209)

// invalidCodeTransientNetwork  is the invalid codeword used for transient test networks.
const invalidCodeTransientNetwork = uint64(0x1cb159857af02018)

// encodeWord encodes a word into a code word.
// In Flow, the word is the account index while the code word
// is the corresponding address.
//
// The function assumes the word is valid (<2^k)
func encodeWord(word uint64) uint64 {
	// Multiply the index GF(2) vector by the code generator matrix
	codeWord := uint64(0)
	for i := 0; i < linearCodeK; i++ {
		if word&1 == 1 {
			codeWord ^= generatorMatrixRows[i]
		}
		word >>= 1
	}
	return codeWord
}

// checks if the input is a valid code word of the linear code
func isValidCodeWord(codeWord uint64) bool {
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

// reverse the linear code assuming the input is a valid
// codeWord.
func decodeCodeWord(codeWord uint64) uint64 {
	// truncate the address GF(2) vector (last K bits) and multiply it by the inverse matrix of
	// the partial code generator.
	word := uint64(0)
	codeWord >>= (linearCodeN - linearCodeK)
	for i := 0; i < linearCodeK; i++ {
		if codeWord&1 == 1 {
			word ^= inverseMatrixRows[i]
		}
		codeWord >>= 1
	}
	return word
}

// Rows of the generator matrix G of the [64,45]-code used for Flow addresses.
// G is a (k x n) matrix with coefficients in GF(2), each row is converted into
// a big endian integer representation of the GF(2) raw vector.
// G is used to generate the account addresses
var generatorMatrixRows = [linearCodeK]uint64{
	0xe467b9dd11fa00df, 0xf233dcee88fe0abe, 0xf919ee77447b7497, 0xfc8cf73ba23a260d,
	0xfe467b9dd11ee2a1, 0xff233dcee888d807, 0xff919ee774476ce6, 0x7fc8cf73ba231d10,
	0x3fe467b9dd11b183, 0x1ff233dcee8f96d6, 0x8ff919ee774757ba, 0x47fc8cf73ba2b331,
	0x23fe467b9dd27f6c, 0x11ff233dceee8e82, 0x88ff919ee775dd8f, 0x447fc8cf73b905e4,
	0xa23fe467b9de0d83, 0xd11ff233dce8d5a7, 0xe88ff919ee73c38a, 0x7447fc8cf73f171f,
	0xba23fe467b9dcb2b, 0xdd11ff233dcb0cb4, 0xee88ff919ee26c5d, 0x77447fc8cf775dd3,
	0x3ba23fe467b9b5a1, 0x9dd11ff233d9117a, 0xcee88ff919efa640, 0xe77447fc8cf3e297,
	0x73ba23fe467fabd2, 0xb9dd11ff233fb16c, 0xdcee88ff919adde7, 0xee77447fc8ceb196,
	0xf73ba23fe4621cd0, 0x7b9dd11ff2379ac3, 0x3dcee88ff91df46c, 0x9ee77447fc88e702,
	0xcf73ba23fe4131b6, 0x67b9dd11ff240f9a, 0x33dcee88ff90f9e0, 0x19ee77447fcff4e3,
	0x8cf73ba23fe64091, 0x467b9dd11ff115c7, 0x233dcee88ffdb735, 0x919ee77447fe2309,
	0xc8cf73ba23fdc736}

// Columns of the parity-check matrix H of the [64,45]-code used for Flow addresses.
// H is a (n x p) matrix with coefficients in GF(2), each column is converted into
// a big endian integer representation of the GF(2) column vector.
// H is used to verify a code word is a valid account address.
var parityCheckMatrixColumns = [linearCodeN]uint{
	0x00001, 0x00002, 0x00004, 0x00008,
	0x00010, 0x00020, 0x00040, 0x00080,
	0x00100, 0x00200, 0x00400, 0x00800,
	0x01000, 0x02000, 0x04000, 0x08000,
	0x10000, 0x20000, 0x40000, 0x7328d,
	0x6689a, 0x6112f, 0x6084b, 0x433fd,
	0x42aab, 0x41951, 0x233ce, 0x22a81,
	0x21948, 0x1ef60, 0x1deca, 0x1c639,
	0x1bdd8, 0x1a535, 0x194ac, 0x18c46,
	0x1632b, 0x1529b, 0x14a43, 0x13184,
	0x12942, 0x118c1, 0x0f812, 0x0e027,
	0x0d00e, 0x0c83c, 0x0b01d, 0x0a831,
	0x0982b, 0x07034, 0x0682a, 0x05819,
	0x03807, 0x007d2, 0x00727, 0x0068e,
	0x0067c, 0x0059d, 0x004eb, 0x003b4,
	0x0036a, 0x002d9, 0x001c7, 0x0003f,
}

// Rows of the inverse I of the generator matrix I = sub(G)^(-1).
// sub(G) is a square sub-matrix of G formed by the first (k) columns. This makes sub(G)
// an inversible (k x k) matrix.
// I is a (k x k) matrix with coefficients in GF(2), each row is converted into
// a big endian integer representation of the GF(2) raw vector.
// I is used to retrieve indices from account addresses.
var inverseMatrixRows = [linearCodeK]uint64{
	0x14b4ae9336c9, 0x1a5a57499b64, 0x0d2d2ba4cdb2, 0x069695d266d9,
	0x134b4ae9336c, 0x09a5a57499b6, 0x04d2d2ba4cdb, 0x1269695d266d,
	0x1934b4ae9336, 0x0c9a5a57499b, 0x164d2d2ba4cd, 0x1b269695d266,
	0x0d934b4ae933, 0x16c9a5a57499, 0x1b64d2d2ba4c, 0x0db269695d26,
	0x06d934b4ae93, 0x136c9a5a5749, 0x19b64d2d2ba4, 0x0cdb269695d2,
	0x066d934b4ae9, 0x1336c9a5a574, 0x099b64d2d2ba, 0x04cdb269695d,
	0x1266d934b4ae, 0x09336c9a5a57, 0x1499b64d2d2b, 0x1a4cdb269695,
	0x1d266d934b4a, 0x0e9336c9a5a5, 0x17499b64d2d2, 0x0ba4cdb26969,
	0x15d266d934b4, 0x0ae9336c9a5a, 0x057499b64d2d, 0x12ba4cdb2696,
	0x095d266d934b, 0x14ae9336c9a5, 0x1a57499b64d2, 0x0d2ba4cdb269,
	0x1695d266d934, 0x0b4ae9336c9a, 0x05a57499b64d, 0x12d2ba4cdb26,
	0x09695d266d93,
}
