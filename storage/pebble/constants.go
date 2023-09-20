package pebble

import "github.com/onflow/flow-go/storage/pebble/registers"

const (
	// lookup keys for register heights (f,l)

	// placeHolderHeight is an element of the height lookup keys of length HeightSuffixLen
	// 10 bits per key yields a filter with <1% false positive rate.
	placeHolderHeight = uint64(0)

	// MinLookupKeyLen is a structure with at least 2 fixed bytes for:
	// 1. '/' byte separator for owner
	// 2. '/' byte for key (owner and key values are blank so both have 0 bytes before each '/')
	// 3. the 8 byte space for uint64 in big endian for the block height
	// we will need to maintain this minimum length to ensure the
	MinLookupKeyLen = 3 + registers.HeightSuffixLen

	// prefixes
	// codeRegister starting at 2, 1 and 0 reserved for DB specific constants
	codeRegister byte = 2
	// codeFirstBlockHeight and  codeLatestBlockHeight keys for register heights
	codeFirstBlockHeight  byte = 3
	codeLatestBlockHeight byte = 4
)
