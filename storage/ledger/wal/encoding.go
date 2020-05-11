package wal

import (
	"encoding/binary"
	"fmt"

	"github.com/dapperlabs/flow-go/model/flow"
)

type WALOperation uint8

const WALUpdate WALOperation = 1
const WALDelete WALOperation = 2

/*
The WAL update record uses two operations so far - an update which must include all keys and values, and deletion
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

// writeShortData writes data shorter than 16kB and returns next free position
func writeShortData(buffer []byte, location int, data []byte) int {
	binary.BigEndian.PutUint16(buffer[location:], uint16(len(data)))
	return writeData(buffer, location+2, data)
}

// readShortData read data shorter than 16kB and returns next free position
func readShortData(buffer []byte, location int) ([]byte, int, error) {
	size := binary.BigEndian.Uint16(buffer[location:])
	return readData(buffer, location+2, int(size))
}

// writeLongData writes data shorter than 32MB and returns next free position
func writeLongData(buffer []byte, location int, data []byte) int {
	binary.BigEndian.PutUint32(buffer[location:], uint32(len(data)))
	return writeData(buffer, location+4, data)
}

// readLongData read data shorter than 32MB and returns next free position
func readLongData(buffer []byte, location int) ([]byte, int, error) {
	size := binary.BigEndian.Uint32(buffer[location:])
	return readData(buffer, location+4, int(size))
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
	binary.BigEndian.PutUint32(buf[pos:], uint32(len(keys)))
	pos += 4

	//key size
	binary.BigEndian.PutUint16(buf[pos:], uint16(len(keys[0])))
	pos += 2

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

	recordsCount := int(binary.BigEndian.Uint32(data[consumed:]))
	consumed += 4

	keySize := int(binary.BigEndian.Uint16(data[consumed:]))
	consumed += 2

	for i := 0; i < recordsCount; i++ {
		expectedBytes := consumed + keySize + 4
		if length < expectedBytes {
			err = fmt.Errorf("data corrupted, too short to represent next value - expected next %d bytes, but got %d", expectedBytes, length)
			return
		}
		var keyData []byte
		var valueData []byte

		keyData, consumed, err = readData(data, consumed, keySize)
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
