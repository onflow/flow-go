package trie

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"sync"

	"github.com/dapperlabs/flow-go/storage/ledger/mtrie/common"
	"github.com/dapperlabs/flow-go/storage/ledger/mtrie/node"
	"github.com/dapperlabs/flow-go/storage/ledger/mtrie/proof"
)

// MTrie is a fully in memory trie with option to persist to disk
type MTrie struct {
	Root           *node.Node
	Number         uint64
	MaxHeight      int
	Values         map[string][]byte
	rootHash       []byte
	ParentRootHash []byte
}

// NewMTrie returns the same Root
func NewMTrie(maxHeight int) *MTrie {
	return &MTrie{Root: node.NewNode(maxHeight - 1),
		Values:    make(map[string][]byte),
		MaxHeight: maxHeight}
}

func (mt *MTrie) StringRootHash() string {
	return hex.EncodeToString(mt.rootHash)
}

func (mt *MTrie) RootHash() []byte {
	return mt.rootHash
}

func (mt *MTrie) SetRootHash(rh []byte) {
	mt.rootHash = rh
}

func (mt *MTrie) String() string {
	trieStr := fmt.Sprintf("Trie Number:%v hash:%v parent: %v\n", mt.Number, mt.StringRootHash(), hex.EncodeToString(mt.ParentRootHash))
	return trieStr + mt.Root.FmtStr("", "")
}

func (mt *MTrie) UnsafeRead(keys [][]byte) ([][]byte, error) {
	return mt.read(mt.Root, keys)
}

func (mt *MTrie) read(head *node.Node, keys [][]byte) ([][]byte, error) {
	// keys not found
	if head == nil {
		res := make([][]byte, 0, len(keys))
		for range keys {
			res = append(res, []byte{})
		}
		return res, nil
	}
	// reached a leaf node
	if head.Key != nil {
		res := make([][]byte, 0)
		for _, k := range keys {
			if bytes.Equal(head.Key, k) {
				res = append(res, head.Value)
			} else {
				res = append(res, []byte{})
			}
		}
		return res, nil
	}

	lkeys, rkeys, err := common.SplitSortedKeys(keys, mt.MaxHeight-head.Height-1)
	if err != nil {
		return nil, fmt.Errorf("can't read due to split Key error: %w", err)
	}

	// TODO make this parallel
	values := make([][]byte, 0)
	if len(lkeys) > 0 {
		v, err := mt.read(head.LChild, lkeys)
		if err != nil {
			return nil, err
		}
		values = append(values, v...)
	}

	if len(rkeys) > 0 {
		v, err := mt.read(head.RChild, rkeys)
		if err != nil {
			return nil, err
		}
		values = append(values, v...)
	}
	return values, nil
}

func (mt *MTrie) UnsafeUpdate(parentTrie *MTrie, keys [][]byte, values [][]byte) error {
	return mt.update(parentTrie.Root, mt.Root, keys, values)
}

func (mt *MTrie) update(parent *node.Node, head *node.Node, keys [][]byte, values [][]byte) error {
	// parent has a Key for this node (add Key and insert)
	if parent.Key != nil {
		alreadyExist := false
		// deduplicate
		for _, k := range keys {
			if bytes.Equal(k, parent.Key) {
				alreadyExist = true
			}
		}
		if !alreadyExist {
			keys = append(keys, parent.Key)
			values = append(values, parent.Value)
		}

	}
	// If we are at a leaf node, we create the node
	if len(keys) == 1 && parent.LChild == nil && parent.RChild == nil {
		head.Key = keys[0]
		head.Value = values[0]
		// ????
		head.HashValue = head.GetNodeHash()
		return nil
	}

	// Split the keys and Values array so we can update the trie in parallel
	lkeys, lvalues, rkeys, rvalues, err := common.SplitKeyValues(keys, values, mt.MaxHeight-head.Height-1)
	if err != nil {
		return fmt.Errorf("error spliting Key Values: %w", err)
	}

	wg := sync.WaitGroup{}
	wg.Add(2)
	var lupdate, rupdate *node.Node
	var err1, err2 error
	go func() {
		defer wg.Done()
		// no change needed on the left side,
		if len(lkeys) == 0 {
			// reuse the node from previous trie
			lupdate = parent.LChild
		} else {
			newN := node.NewNode(parent.Height - 1)
			if parent.LChild != nil {
				err1 = mt.update(parent.LChild, newN, lkeys, lvalues)
			} else {
				err1 = mt.update(node.NewNode(parent.Height-1), newN, lkeys, lvalues)
			}
			newN.PopulateNodeHashValues()
			lupdate = newN
		}
	}()
	go func() {
		defer wg.Done()
		// no change needed on right side
		if len(rkeys) == 0 {
			// reuse the node from previous trie
			rupdate = parent.RChild
		} else {
			newN := node.NewNode(head.Height - 1)
			if parent.RChild != nil {
				err2 = mt.update(parent.RChild, newN, rkeys, rvalues)
			} else {
				err2 = mt.update(node.NewNode(head.Height-1), newN, rkeys, rvalues)
			}
			newN.PopulateNodeHashValues()
			rupdate = newN
		}
	}()
	wg.Wait()

	if err1 != nil {
		return err1
	}
	head.LChild = lupdate

	if err2 != nil {
		return err2
	}
	head.RChild = rupdate

	return nil
}

func (mt *MTrie) UnsafeProofs(keys [][]byte, proofs []*proof.Proof) error {
	return mt.proofs(mt.Root, keys, proofs)
}

func (mt *MTrie) proofs(head *node.Node, keys [][]byte, proofs []*proof.Proof) error {
	// we've reached the end of a trie
	// and Key is not found (noninclusion proof)
	if head == nil {
		return nil
	}

	// we've reached a leaf that has a Key
	if head.Key != nil {
		// Value matches (inclusion proof)
		if bytes.Equal(head.Key, keys[0]) {
			proofs[0].Inclusion = true
		}
		return nil
	}

	// increment steps for all the proofs
	for _, p := range proofs {
		p.Steps++
	}
	// split keys based on the Value of i-th bit (i = trie Height - node Height)
	lkeys, lproofs, rkeys, rproofs, err := proof.SplitKeyProofs(keys, proofs, mt.MaxHeight-head.Height-1)
	if err != nil {
		return fmt.Errorf("proof generation failed, split Key error: %w", err)
	}

	if len(lkeys) > 0 {
		if head.RChild != nil {
			nodeHash := head.RChild.GetNodeHash()
			isDef := bytes.Equal(nodeHash, common.GetDefaultHashForHeight(head.RChild.Height))
			for _, p := range lproofs {
				// we skip default Values
				if !isDef {
					err := common.SetBit(p.Flags, mt.MaxHeight-head.Height-1)
					if err != nil {
						return err
					}
					p.Values = append(p.Values, nodeHash)
				}
			}
		}
		err := mt.proofs(head.LChild, lkeys, lproofs)
		if err != nil {
			return err
		}
	}

	if len(rkeys) > 0 {
		if head.LChild != nil {
			nodeHash := head.LChild.GetNodeHash()
			isDef := bytes.Equal(nodeHash, common.GetDefaultHashForHeight(head.LChild.Height))
			for _, p := range rproofs {
				// we skip default Values
				if !isDef {
					err := common.SetBit(p.Flags, mt.MaxHeight-head.Height-1)
					if err != nil {
						return err
					}
					p.Values = append(p.Values, nodeHash)
				}
			}
		}
		err := mt.proofs(head.RChild, rkeys, rproofs)
		if err != nil {
			return err
		}
	}
	return nil
}

// Store stores the trie Key Values to a file
func (mt *MTrie) Store(path string) error {
	fi, err := os.Create(path)
	if err != nil {
		return err
	}
	defer fi.Close()
	writer := bufio.NewWriter(fi)
	defer writer.Flush()

	// first byte is the store format version
	_, err = writer.Write([]byte{byte(1)})
	if err != nil {
		return err
	}

	// then 8 bytes captures trie Number
	b1 := make([]byte, 8)
	binary.LittleEndian.PutUint64(b1, mt.Number)
	_, err = writer.Write(b1)
	if err != nil {
		return err
	}

	// then 2 byte2 captures the MaxHeight
	b2 := make([]byte, 2)
	binary.LittleEndian.PutUint16(b2, uint16(mt.MaxHeight))
	_, err = writer.Write(b2)
	if err != nil {
		return err
	}

	// next 32 bytes are parent rootHash
	_, err = writer.Write(mt.ParentRootHash)
	if err != nil {
		return err
	}

	// next 32 bytes are trie rootHash
	_, err = writer.Write(mt.rootHash)
	if err != nil {
		return err
	}

	// repeated: x bytes Key, 4bytes valueSize(Number of bytes Value took), valueSize bytes Value)
	err = mt.store(mt.Root, writer)
	if err != nil {
		return err
	}

	return nil
}

func (mt *MTrie) store(n *node.Node, writer *bufio.Writer) error {
	if n.Key != nil {
		_, err := writer.Write(n.Key)
		if err != nil {
			return err
		}

		b := make([]byte, 8)
		binary.LittleEndian.PutUint64(b, uint64(len(n.Value)))
		_, err = writer.Write(b)
		if err != nil {
			return err
		}

		_, err = writer.Write(n.Value)
		if err != nil {
			return err
		}
	}

	if n.LChild != nil {
		err := mt.store(n.LChild, writer)
		if err != nil {
			return err
		}
	}

	if n.RChild != nil {
		err := mt.store(n.RChild, writer)
		if err != nil {
			return err
		}
	}
	return nil
}

// Load loads a trie
func (mt *MTrie) Load(path string) error {

	keyByteSize := (mt.MaxHeight - 1) / 8
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

	// next 8 bytes captures the Key size
	numberB := make([]byte, 8)
	_, err = fi.Read(numberB)
	if err != nil {
		return err
	}
	mt.Number = binary.LittleEndian.Uint64(numberB)

	// next 2 bytes captures the MaxHeight
	maxHeightB := make([]byte, 2)
	_, err = fi.Read(maxHeightB)
	if err != nil {
		return err
	}
	maxHeight := binary.LittleEndian.Uint16(maxHeightB)

	// assert max Height
	if maxHeight != uint16(mt.MaxHeight) {
		return errors.New("MaxHeight doesn't match")
	}

	// next 32 bytes are parent rootHash
	parentRootHash := make([]byte, 32)
	_, err = fi.Read(parentRootHash)
	if err != nil {
		return err
	}
	mt.ParentRootHash = parentRootHash

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

	return mt.load(mt.Root, 0, keys, values)
}

func (mt *MTrie) load(n *node.Node, level int, keys [][]byte, values [][]byte) error {
	if len(keys) == 1 {
		n.Key = keys[0]
		n.Value = values[0]
		n.HashValue = common.ComputeCompactValue(n.Key, n.Value, n.Height)
		return nil
	}
	// TODO optimize as keys are already sorted
	lkeys, lvalues, rkeys, rvalues, err := common.SplitKeyValues(keys, values, level)
	if err != nil {
		return err
	}
	if len(lkeys) > 0 {
		n.LChild = node.NewNode(n.Height - 1)
		err := mt.load(n.LChild, level+1, lkeys, lvalues)
		if err != nil {
			return err
		}
	}

	if len(rkeys) > 0 {
		n.RChild = node.NewNode(n.Height - 1)
		err := mt.load(n.RChild, level+1, rkeys, rvalues)
		if err != nil {
			return err
		}
	}
	return nil
}
