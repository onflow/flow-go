package wal

import (
	"encoding/binary"
	"fmt"
	"io"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/storage/ledger/mtrie"
)

type WALOperation uint8

const WALUpdate WALOperation = 1
const WALDelete WALOperation = 2

/*
The LedgerWAL update record uses two operations so far - an update which must include all keys and values, and deletion
which only needs a root tree state commitment.
Updates need to be atomic, hence we prepare binary representation of whole changeset.
Since keys, values and state commitments date types are variable length, we have to store it as well.
Every record has:

1 byte Operation Type | 2 bytes Big Endian uint16 length of state commitment | state commitment data

If OP = WALUpdate, then it follow with:

4 bytes Big Endian uint32 - total number of key/value pairs | 2 bytes Big Endian uint16 - length of key (keys are the same length)

and for every pair after
bytes for key | 4 bytes Big Endian uint32 - length of value | value bytes

The code here is deliberately simple, for performance.

*/

func headerSize(stateCommitment flow.StateCommitment) int {
	return 1 + 2 + len(stateCommitment) //byte to encode operation, 2 bytes for encoding length
}

func updateSize(stateCommitment flow.StateCommitment, keys [][]byte, values [][]byte) int {
	size := headerSize(stateCommitment) + 4 + 2 //records count + key size

	for i := range keys {
		size += len(keys[i]) + 4 + len(values[i]) //2 bytes to encode value length
	}

	return size
}

func deleteSize(stateCommitment flow.StateCommitment) int {
	return headerSize(stateCommitment)
}

func writeUint16(buffer []byte, location int, value uint16) int {
	binary.BigEndian.PutUint16(buffer[location:], value)
	return location + 2
}

func readUint16(buffer []byte, location int) (uint16, int) {
	value := binary.BigEndian.Uint16(buffer[location:])
	return value, location + 2
}

func readUint32(buffer []byte, location int) (uint32, int) {
	value := binary.BigEndian.Uint32(buffer[location:])
	return value, location + 4
}

func readUint64(buffer []byte, location int) (uint64, int) {
	value := binary.BigEndian.Uint64(buffer[location:])
	return value, location + 8
}

func writeUint32(buffer []byte, location int, value uint32) int {
	binary.BigEndian.PutUint32(buffer[location:], value)
	return location + 4
}

func writeUint64(buffer []byte, location int, value uint64) int {
	binary.BigEndian.PutUint64(buffer[location:], value)
	return location + 8
}

// writeShortData writes data shorter than 16kB and returns next free position
func writeShortData(buffer []byte, location int, data []byte) int {
	location = writeUint16(buffer, location, uint16(len(data)))
	return writeData(buffer, location, data)
}

func readFromBuffer(reader io.Reader, length int) ([]byte, error) {
	if length == 0 {
		return nil, nil
	}
	buf := make([]byte, length)
	_, err := io.ReadFull(reader, buf)
	if err != nil {
		return nil, fmt.Errorf("cannot read data: %w", err)
	}
	return buf, nil
}

// readShortData read data shorter than 16kB and returns next free position
func readShortData(buffer []byte, location int) ([]byte, int, error) {
	size, location := readUint16(buffer, location)
	return readData(buffer, location, int(size))
}

// readShortDataFromReader reads data shorter than 16kB from reader
func readShortDataFromReader(reader io.Reader) ([]byte, error) {
	buf, err := readFromBuffer(reader, 2)
	if err != nil {
		return nil, fmt.Errorf("cannot read short data length: %w", err)
	}
	size, _ := readUint16(buf, 0)

	buf, err = readFromBuffer(reader, int(size))
	if err != nil {
		return nil, fmt.Errorf("cannot read short data: %w", err)
	}

	return buf, nil
}

// readLongDataFromReader reads data shorter than 16kB from reader
func readLongDataFromReader(reader io.Reader) ([]byte, error) {
	buf, err := readFromBuffer(reader, 4)
	if err != nil {
		return nil, fmt.Errorf("cannot read long data length: %w", err)
	}
	size, _ := readUint32(buf, 0)

	buf, err = readFromBuffer(reader, int(size))
	if err != nil {
		return nil, fmt.Errorf("cannot read long data: %w", err)
	}

	return buf, nil
}

// writeLongData writes data shorter than 32MB and returns next free position
func writeLongData(buffer []byte, location int, data []byte) int {
	location = writeUint32(buffer, location, uint32(len(data)))
	return writeData(buffer, location, data)
}

// readLongData read data shorter than 32MB and returns next free position
func readLongData(buffer []byte, location int) ([]byte, int, error) {
	size, location := readUint32(buffer, location)
	return readData(buffer, location, int(size))
}

// writeData writes data directly and returns next free position
func writeData(buffer []byte, location int, data []byte) int {
	copy(buffer[location:], data)
	return location + len(data)
}

// readData reads data directly returns next free position
func readData(buffer []byte, location int, size int) ([]byte, int, error) {
	data := make([]byte, size)
	copied := copy(data, buffer[location:])
	if copied != size {
		return nil, 0, fmt.Errorf("read data count mismatch. Expected %d but read %d", size, copied)
	}
	return data, location + len(data), nil
}

// writeHeader writes header and return next free position
func writeHeader(buffer []byte, op WALOperation, stateCommitment flow.StateCommitment) int {
	buffer[0] = byte(op)
	return writeShortData(buffer, 1, stateCommitment)
}

func EncodeUpdate(stateCommitment flow.StateCommitment, keys [][]byte, values [][]byte) []byte {

	buf := make([]byte, updateSize(stateCommitment, keys, values))

	pos := writeHeader(buf, WALUpdate, stateCommitment)

	//number of records
	pos = writeUint32(buf, pos, uint32(len(keys)))

	//key size
	pos = writeUint16(buf, pos, uint16(len(keys[0])))

	for i := range keys {
		pos = writeData(buf, pos, keys[i])
		pos = writeLongData(buf, pos, values[i])
	}

	return buf
}

func EncodeDelete(stateCommitment flow.StateCommitment) []byte {

	buf := make([]byte, deleteSize(stateCommitment))

	_ = writeHeader(buf, WALDelete, stateCommitment)

	return buf
}

func Decode(data []byte) (operation WALOperation, stateCommitment flow.StateCommitment, keys [][]byte, values [][]byte, err error) {
	length := len(data)
	consumed := 0
	if length < 4 { // 1 byte op + 2 size + actual data = 4 minimum
		err = fmt.Errorf("data corrupted, too short to represent operation - hexencoded data: %x", data)
		return
	}
	switch data[0] {
	case byte(WALUpdate), byte(WALDelete):
		operation = WALOperation(data[0])
	default:
		err = fmt.Errorf("unknown operation type, given: %x", data[0])
		return
	}
	consumed++

	stateCommitment, consumed, err = readShortData(data, consumed)
	if err != nil {
		err = fmt.Errorf("cannot read state commitment: %w", err)
		return
	}

	if operation == WALDelete {
		return
	}

	if length < consumed+6 {
		err = fmt.Errorf("data corrupted, too short to represent update operation - hexencoded data: %x", data)
		return
	}

	//recordsCount := int(binary.BigEndian.Uint32(data[consumed:]))
	//consumed += 4
	recordsCount, consumed := readUint32(data, consumed)

	//keySize := int(binary.BigEndian.Uint16(data[consumed:]))
	//consumed += 2
	keySize, consumed := readUint16(data, consumed)

	for i := 0; i < int(recordsCount); i++ {
		expectedBytes := consumed + int(keySize) + 4
		if length < expectedBytes {
			err = fmt.Errorf("data corrupted, too short to represent next value - expected next %d bytes, but got %d", expectedBytes, length)
			return
		}
		var keyData []byte
		var valueData []byte

		keyData, consumed, err = readData(data, consumed, int(keySize))
		if err != nil {
			err = fmt.Errorf("error while reading key data: %w", err)
			return
		}

		valueData, consumed, err = readLongData(data, consumed)
		if err != nil {
			err = fmt.Errorf("error while reading value data: %w", err)
			return
		}

		keys = append(keys, keyData)
		values = append(values, valueData)
	}

	return
}

// EncodeStorableNode encodes StorableNode
// 2-bytes Big Endian uint16 height
// 8-bytes Big Endian uint64 LIndex
// 8-bytes Big Endian uint64 RIndex
// 2-bytes Big Endian uint16 key length
// key bytes
// 4-bytes Big Endian uint32 value length
// value bytes
// 2-bytes Big Endian uint16 hashValue length
// hash value bytes
func EncodeStorableNode(storableNode *mtrie.StorableNode) []byte {

	length := 2 + 8 + 8 + 2 + len(storableNode.Key) + 4 + len(storableNode.Value) + 2 + len(storableNode.HashValue)

	buf := make([]byte, length)
	pos := 0

	pos = writeUint16(buf, pos, storableNode.Height)
	pos = writeUint64(buf, pos, storableNode.LIndex)
	pos = writeUint64(buf, pos, storableNode.RIndex)
	pos = writeShortData(buf, pos, storableNode.Key)
	pos = writeLongData(buf, pos, storableNode.Value)
	writeShortData(buf, pos, storableNode.HashValue)

	return buf
}

func ReadStorableNode(reader io.Reader) (*mtrie.StorableNode, error) {

	buf := make([]byte, 2+8+8)

	_, err := io.ReadFull(reader, buf)
	if err != nil {
		return nil, fmt.Errorf("cannot read fixed-legth part: %w", err)
	}

	pos := 0

	storableNode := &mtrie.StorableNode{}

	storableNode.Height, pos = readUint16(buf, pos)
	storableNode.LIndex, pos = readUint64(buf, pos)
	storableNode.RIndex, pos = readUint64(buf, pos)

	storableNode.Key, err = readShortDataFromReader(reader)
	if err != nil {
		return nil, fmt.Errorf("cannot read key data: %w", err)
	}
	storableNode.Value, err = readLongDataFromReader(reader)
	if err != nil {
		return nil, fmt.Errorf("cannot read value data: %w", err)
	}
	storableNode.HashValue, err = readShortDataFromReader(reader)
	if err != nil {
		return nil, fmt.Errorf("cannot read hashValue data: %w", err)
	}

	return storableNode, nil
}

// EncodeStorableTrie encodes StorableTrie
// 2-bytes Big Endian uint16 MaxHeight
// 8-bytes Big Endian uint64 Number
// 8-bytes Big Endian uint64 RootIndex
// 2-bytes Big Endian uint16 RootHash length
// RootHash bytes
// 2-bytes Big Endian uint16 ParentRootHash length
// ParentRootHash bytes
func EncodeStorableTrie(storableTrie *mtrie.StorableTrie) []byte {
	length := 2 + 8 + 8 + 2 + len(storableTrie.RootHash) + 2 + len(storableTrie.ParentRootHash)

	buf := make([]byte, length)

	pos := 0

	pos = writeUint16(buf, pos, storableTrie.MaxHeight)
	pos = writeUint64(buf, pos, storableTrie.Number)
	pos = writeUint64(buf, pos, storableTrie.RootIndex)
	pos = writeShortData(buf, pos, storableTrie.RootHash)
	pos = writeShortData(buf, pos, storableTrie.ParentRootHash)

	return buf
}

func ReadStorableTrie(reader io.Reader) (*mtrie.StorableTrie, error) {

	buf := make([]byte, 2+8+8)

	read, err := reader.Read(buf)
	if err != nil {
		return nil, fmt.Errorf("cannot read fixed-legth part: %w", err)
	}
	if read != len(buf) {
		return nil, fmt.Errorf("not enough bytes read %d expected %d", read, len(buf))
	}

	pos := 0

	storableNode := &mtrie.StorableTrie{}

	storableNode.MaxHeight, pos = readUint16(buf, pos)
	storableNode.Number, pos = readUint64(buf, pos)
	storableNode.RootIndex, pos = readUint64(buf, pos)

	storableNode.RootHash, err = readShortDataFromReader(reader)
	if err != nil {
		return nil, fmt.Errorf("cannot read roothash data: %w", err)
	}
	storableNode.ParentRootHash, err = readShortDataFromReader(reader)
	if err != nil {
		return nil, fmt.Errorf("cannot read parentRootHash data: %w", err)
	}

	return storableNode, nil
}
