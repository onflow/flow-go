package pebble

import "github.com/onflow/flow-go/storage/pebble/registers"

const (
	// checkpointLeafNodeBufSize is the batch size of leaf nodes being read from the checkpoint file,
	// for use by wal.OpenAndReadLeafNodesFromCheckpointV6
	checkpointLeafNodeBufSize = 1000

	// pebbleBootstrapRegisterBatchLen is the batch size of converted register values to be written to pebble by the
	// register bootstrap process
	pebbleBootstrapRegisterBatchLen = 1000

	// placeHolderHeight is an element of the height lookup keys of length HeightSuffixLen
	// 10 bits per key yields a filter with <1% false positive rate.
	placeHolderHeight = uint64(0)

	// MinLookupKeyLen defines the minimum length for a valid lookup key
	//
	// Lookup keys use the following format:
	//     [code] [owner] / [key] / [height]
	// Where:
	// - code: 1 byte indicating the type of data stored
	// - owner: optional variable length field
	// - key: optional variable length field
	// - height: 8 bytes representing the block height (uint64)
	// - separator: '/' is used to separate variable length fields (required 2)
	//
	// Therefore the minimum key would be 3 bytes + # of bytes for height
	//     [code] / / [height]
	MinLookupKeyLen = 3 + registers.HeightSuffixLen

	// prefixes
	// codeRegister starting at 2, 1 and 0 reserved for DB specific constants
	codeRegister byte = 2
	// codeFirstBlockHeight and codeLatestBlockHeight are keys for the range of block heights in the register store
	codeFirstBlockHeight  byte = 3
	codeLatestBlockHeight byte = 4
)
