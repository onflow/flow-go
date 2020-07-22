package flattener

import (
	"fmt"
	"io"

	"github.com/dapperlabs/flow-go/ledger/common"
)

const encodingDecodingVersion = uint16(0)

// EncodeStorableNode encodes StorableNode
func EncodeStorableNode(storableNode *StorableNode) []byte {

	length := 2 + 2 + 8 + 8 + 2 + 8 + 2 + len(storableNode.Path) + 4 + len(storableNode.EncPayload) + 2 + len(storableNode.HashValue)
	buf := make([]byte, 0, length)
	// 2-bytes encoding version
	buf = common.AppendUint16(buf, encodingDecodingVersion)

	// 2-bytes Big Endian uint16 height
	buf = common.AppendUint16(buf, storableNode.Height)

	// 8-bytes Big Endian uint64 LIndex
	buf = common.AppendUint64(buf, storableNode.LIndex)

	// 8-bytes Big Endian uint64 RIndex
	buf = common.AppendUint64(buf, storableNode.RIndex)

	// 2-bytes Big Endian maxDepth
	buf = common.AppendUint16(buf, storableNode.MaxDepth)

	// 8-bytes Big Endian regCount
	buf = common.AppendUint64(buf, storableNode.RegCount)

	// 2-bytes Big Endian uint16 encoded path length and n-bytes encoded path
	buf = common.AppendShortData(buf, storableNode.Path)

	// 4-bytes Big Endian uint32 encoded payload length and n-bytes encoded payload
	buf = common.AppendLongData(buf, storableNode.EncPayload)

	// 2-bytes Big Endian uint16 hashValue length and n-bytes hashValue
	buf = common.AppendShortData(buf, storableNode.HashValue)

	return buf
}

// ReadStorableNode reads a storable node from io
func ReadStorableNode(reader io.Reader) (*StorableNode, error) {

	// reading version
	buf := make([]byte, 2)
	read, err := io.ReadFull(reader, buf)
	if err != nil {
		return nil, fmt.Errorf("error reading storable node, cannot read version part: %w", err)
	}
	if read != len(buf) {
		return nil, fmt.Errorf("not enough bytes read %d expected %d", read, len(buf))
	}

	version, _, err := common.ReadUint16(buf)
	if err != nil {
		return nil, fmt.Errorf("error reading storable node: %w", err)
	}

	if version > encodingDecodingVersion {
		return nil, fmt.Errorf("error reading storable node: unsuported version %d > %d", version, encodingDecodingVersion)
	}

	// reading fixed-length part
	buf = make([]byte, 2+8+8+2+8)

	read, err = io.ReadFull(reader, buf)
	if err != nil {
		return nil, fmt.Errorf("error reading storable node, cannot read fixed-length part: %w", err)
	}
	if read != len(buf) {
		return nil, fmt.Errorf("not enough bytes read %d expected %d", read, len(buf))
	}

	storableNode := &StorableNode{}

	storableNode.Height, buf, err = common.ReadUint16(buf)
	if err != nil {
		return nil, fmt.Errorf("error reading storable node: %w", err)
	}

	storableNode.LIndex, buf, err = common.ReadUint64(buf)
	if err != nil {
		return nil, fmt.Errorf("error reading storable node: %w", err)
	}

	storableNode.RIndex, buf, err = common.ReadUint64(buf)
	if err != nil {
		return nil, fmt.Errorf("error reading storable node: %w", err)
	}

	storableNode.MaxDepth, buf, err = common.ReadUint16(buf)
	if err != nil {
		return nil, fmt.Errorf("error reading storable node: %w", err)
	}

	storableNode.RegCount, _, err = common.ReadUint64(buf)
	if err != nil {
		return nil, fmt.Errorf("error reading storable node: %w", err)
	}

	storableNode.Path, err = common.ReadShortDataFromReader(reader)
	if err != nil {
		return nil, fmt.Errorf("cannot read key data: %w", err)
	}

	storableNode.EncPayload, err = common.ReadLongDataFromReader(reader)
	if err != nil {
		return nil, fmt.Errorf("cannot read value data: %w", err)
	}

	storableNode.HashValue, err = common.ReadShortDataFromReader(reader)
	if err != nil {
		return nil, fmt.Errorf("cannot read hashValue data: %w", err)
	}

	return storableNode, nil
}

// EncodeStorableTrie encodes StorableTrie
func EncodeStorableTrie(storableTrie *StorableTrie) []byte {
	length := 8 + 2 + len(storableTrie.RootHash)
	buf := make([]byte, 0, length)
	// 2-bytes encoding version
	buf = common.AppendUint16(buf, encodingDecodingVersion)

	// 8-bytes Big Endian uint64 RootIndex
	buf = common.AppendUint64(buf, storableTrie.RootIndex)

	// 2-bytes Big Endian uint16 RootHash length and n-bytes RootHash
	buf = common.AppendShortData(buf, storableTrie.RootHash)

	return buf
}

// ReadStorableTrie reads a storable trie from io
func ReadStorableTrie(reader io.Reader) (*StorableTrie, error) {
	storableTrie := &StorableTrie{}

	// reading version
	buf := make([]byte, 2)
	read, err := io.ReadFull(reader, buf)
	if err != nil {
		return nil, fmt.Errorf("error reading storable node, cannot read version part: %w", err)
	}
	if read != len(buf) {
		return nil, fmt.Errorf("not enough bytes read %d expected %d", read, len(buf))
	}

	version, _, err := common.ReadUint16(buf)
	if err != nil {
		return nil, fmt.Errorf("error reading storable node: %w", err)
	}

	if version > encodingDecodingVersion {
		return nil, fmt.Errorf("error reading storable node: unsuported version %d > %d", version, encodingDecodingVersion)
	}

	// read root uint64 RootIndex
	buf = make([]byte, 8)
	read, err = io.ReadFull(reader, buf)
	if err != nil {
		return nil, fmt.Errorf("cannot read fixed-legth part: %w", err)
	}
	if read != len(buf) {
		return nil, fmt.Errorf("not enough bytes read %d expected %d", read, len(buf))
	}

	rootIndex, _, err := common.ReadUint64(buf)
	if err != nil {
		return nil, fmt.Errorf("cannot read root index data: %w", err)
	}
	storableTrie.RootIndex = rootIndex

	roothash, err := common.ReadShortDataFromReader(reader)
	if err != nil {
		return nil, fmt.Errorf("cannot read roothash data: %w", err)
	}
	storableTrie.RootHash = roothash

	return storableTrie, nil
}
