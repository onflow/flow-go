package mtrie

import (
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
)

// MTrie is a fully in memory trie with option to persist to disk
type MTrie struct {
	root           *node
	number         uint64
	maxHeight      int
	values         map[string][]byte
	rootHash       []byte
	parentRootHash []byte
}

// NewMTrie returns the same root
func NewMTrie(maxHeight int) *MTrie {
	return &MTrie{root: newNode(maxHeight - 1),
		values:    make(map[string][]byte),
		maxHeight: maxHeight}
}

func (mt *MTrie) String() string {
	trieStr := fmt.Sprintf("Trie number:%v hash:%v parent: %v\n", mt.number, hex.EncodeToString(mt.rootHash), hex.EncodeToString(mt.parentRootHash))
	return trieStr + mt.root.FmtStr("", "")
}

// Store stores the trie key values to a file
func (mt *MTrie) Store(path string) error {
	fi, err := os.Create(path)
	if err != nil {
		return err
	}
	defer fi.Close()

	// first byte is the store format version
	_, err = fi.Write([]byte{byte(1)})
	if err != nil {
		return err
	}

	// then 8 bytes captures trie number
	b1 := make([]byte, 8)
	binary.LittleEndian.PutUint64(b1, mt.number)
	_, err = fi.Write(b1)
	if err != nil {
		return err
	}

	// then 2 byte2 captures the maxHeight
	b2 := make([]byte, 2)
	binary.LittleEndian.PutUint16(b2, uint16(mt.maxHeight))
	_, err = fi.Write(b2)
	if err != nil {
		return err
	}

	// next 32 bytes are parent rootHash
	_, err = fi.Write(mt.parentRootHash)
	if err != nil {
		return err
	}

	// next 32 bytes are trie rootHash
	_, err = fi.Write(mt.rootHash)
	if err != nil {
		return err
	}

	// repeated: x bytes key, 4bytes valueSize(number of bytes value took), valueSize bytes value)
	err = mt.store(mt.root, fi)
	if err != nil {
		return err
	}

	return nil
}

func (mt *MTrie) store(n *node, fi *os.File) error {
	if n.key != nil {
		_, err := fi.Write(n.key)
		if err != nil {
			return err
		}

		b := make([]byte, 8)
		binary.LittleEndian.PutUint64(b, uint64(len(n.value)))
		_, err = fi.Write(b)
		if err != nil {
			return err
		}

		_, err = fi.Write(n.value)
		if err != nil {
			return err
		}
	}

	if n.lChild != nil {
		err := mt.store(n.lChild, fi)
		if err != nil {
			return err
		}
	}

	if n.rChild != nil {
		err := mt.store(n.rChild, fi)
		if err != nil {
			return err
		}
	}
	return nil
}

// Load loads a trie
func (mt *MTrie) Load(path string) error {

	keyByteSize := (mt.maxHeight - 1) / 8
	fi, err := os.Open(path)
	if err != nil {
		return err
	}
	defer fi.Close()

	// first byte is the store format version
	version := make([]byte, 1)
	_, err = fi.Read(version)
	if err != nil {
		return err
	}

	// assert encoding version
	if uint8(version[0]) != uint8(1) {
		return errors.New("trie store/load version doesn't match")
	}

	// next 8 bytes captures the key size
	numberB := make([]byte, 8)
	_, err = fi.Read(numberB)
	if err != nil {
		return err
	}
	mt.number = binary.LittleEndian.Uint64(numberB)

	// next 2 bytes captures the maxHeight
	maxHeightB := make([]byte, 2)
	_, err = fi.Read(maxHeightB)
	if err != nil {
		return err
	}
	maxHeight := binary.LittleEndian.Uint16(maxHeightB)

	// assert max height
	if maxHeight != uint16(mt.maxHeight) {
		return errors.New("maxHeight doesn't match")
	}

	// next 32 bytes are parent rootHash
	parentRootHash := make([]byte, 32)
	_, err = fi.Read(parentRootHash)
	if err != nil {
		return err
	}
	mt.parentRootHash = parentRootHash

	// next 32 bytes are rootHash
	rootHash := make([]byte, 32)
	_, err = fi.Read(rootHash)
	if err != nil {
		return err
	}
	mt.rootHash = rootHash

	keys := make([][]byte, 0)
	values := make([][]byte, 0)

	for {
		key := make([]byte, keyByteSize)
		_, err = fi.Read(key)
		if err != nil {
			if err == io.EOF {
				break
			}
			return err
		}

		valueSizeB := make([]byte, 8)
		_, err = fi.Read(valueSizeB)
		if err != nil {
			return err
		}

		valueSize := binary.LittleEndian.Uint64(valueSizeB)
		value := make([]byte, valueSize)
		_, err = fi.Read(value)
		if err != nil {
			return err
		}

		keys = append(keys, key)
		values = append(values, value)
	}

	return mt.load(mt.root, 0, keys, values)
}

func (mt *MTrie) load(n *node, level int, keys [][]byte, values [][]byte) error {
	if len(keys) == 1 {
		n.key = keys[0]
		n.value = values[0]
		n.hashValue = ComputeCompactValue(n.key, n.value, n.height, mt.maxHeight)
		return nil
	}
	// TODO optimize as keys are already sorted
	lkeys, lvalues, rkeys, rvalues, err := SplitKeyValues(keys, values, level)
	if err != nil {
		return err
	}
	if len(lkeys) > 0 {
		n.lChild = newNode(n.height - 1)
		err := mt.load(n.lChild, level+1, lkeys, lvalues)
		if err != nil {
			return err
		}
	}

	if len(rkeys) > 0 {
		n.rChild = newNode(n.height - 1)
		err := mt.load(n.rChild, level+1, rkeys, rvalues)
		if err != nil {
			return err
		}
	}
	return nil
}
