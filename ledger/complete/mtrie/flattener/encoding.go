package flattener

import (
	"fmt"
	"io"

	"github.com/onflow/flow-go/ledger/common/utils"
)

const encodingDecodingVersion = uint16(0)

// EncodeStorableNode encodes StorableNode
func EncodeStorableNode(storableNode *StorableNode) []byte {

	length := 2 + 2 + 8 + 8 + 2 + 8 + 2 + len(storableNode.Path) + 4 + len(storableNode.EncPayload) + 2 + len(storableNode.HashValue)
	buf := make([]byte, 0, length)
	// 2-bytes encoding version
	buf = utils.AppendUint16(buf, encodingDecodingVersion)

	// 2-bytes Big Endian uint16 height
	buf = utils.AppendUint16(buf, storableNode.Height)

	// 8-bytes Big Endian uint64 LIndex
	buf = utils.AppendUint64(buf, storableNode.LIndex)

	// 8-bytes Big Endian uint64 RIndex
	buf = utils.AppendUint64(buf, storableNode.RIndex)

	// 2-bytes Big Endian maxDepth
	buf = utils.AppendUint16(buf, storableNode.MaxDepth)

	// 8-bytes Big Endian regCount
	buf = utils.AppendUint64(buf, storableNode.RegCount)

	// 2-bytes Big Endian uint16 encoded path length and n-bytes encoded path
	buf = utils.AppendShortData(buf, storableNode.Path)

	// 4-bytes Big Endian uint32 encoded payload length and n-bytes encoded payload
	buf = utils.AppendLongData(buf, storableNode.EncPayload)

	// 2-bytes Big Endian uint16 hashValue length and n-bytes hashValue
	buf = utils.AppendShortData(buf, storableNode.HashValue)

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

	version, _, err := utils.ReadUint16(buf)
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

	storableNode.Height, buf, err = utils.ReadUint16(buf)
	if err != nil {
		return nil, fmt.Errorf("error reading storable node: %w", err)
	}

	storableNode.LIndex, buf, err = utils.ReadUint64(buf)
	if err != nil {
		return nil, fmt.Errorf("error reading storable node: %w", err)
	}

	storableNode.RIndex, buf, err = utils.ReadUint64(buf)
	if err != nil {
		return nil, fmt.Errorf("error reading storable node: %w", err)
	}

	storableNode.MaxDepth, buf, err = utils.ReadUint16(buf)
	if err != nil {
		return nil, fmt.Errorf("error reading storable node: %w", err)
	}

	storableNode.RegCount, _, err = utils.ReadUint64(buf)
	if err != nil {
		return nil, fmt.Errorf("error reading storable node: %w", err)
	}

	storableNode.Path, err = utils.ReadShortDataFromReader(reader)
	if err != nil {
		return nil, fmt.Errorf("cannot read key data: %w", err)
	}

	storableNode.EncPayload, err = utils.ReadLongDataFromReader(reader)
	if err != nil {
		return nil, fmt.Errorf("cannot read value data: %w", err)
	}

	storableNode.HashValue, err = utils.ReadShortDataFromReader(reader)
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
	buf = utils.AppendUint16(buf, encodingDecodingVersion)

	// 8-bytes Big Endian uint64 RootIndex
	buf = utils.AppendUint64(buf, storableTrie.RootIndex)

	// 2-bytes Big Endian uint16 RootHash length and n-bytes RootHash
	buf = utils.AppendShortData(buf, storableTrie.RootHash)

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

	version, _, err := utils.ReadUint16(buf)
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

	rootIndex, _, err := utils.ReadUint64(buf)
	if err != nil {
		return nil, fmt.Errorf("cannot read root index data: %w", err)
	}
	storableTrie.RootIndex = rootIndex

	roothash, err := utils.ReadShortDataFromReader(reader)
	if err != nil {
		return nil, fmt.Errorf("cannot read roothash data: %w", err)
	}
	storableTrie.RootHash = roothash

	return storableTrie, nil
}
