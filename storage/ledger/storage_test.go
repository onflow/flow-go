package ledger

import (
	"bytes"
	"os"
	"testing"

	"github.com/dapperlabs/flow-go/storage/ledger/utils"
)

const (
	// kvdbPath is the path to the key-value database.
	kvdbPath = "db/valuedb"
	// tdbPath is the path to the trie database.
	tdbPath = "db/triedb"
)

func TestNewTrieStorage(t *testing.T) {
	f, err := NewTrieStorage(kvdbPath, tdbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll("./db/")
	f.CloseStorage()
}

func TestTrieTrusted(t *testing.T) {
	f, err := NewTrieStorage(kvdbPath, tdbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll("./db/")

	keys, values := makeTestKeys()

	newRoot, err := f.UpdateRegisters(keys, values)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(f.tree.GetRoot().GetValue(), newRoot) {
		t.Fatalf("Something in UpdateRegister went wrong")
	}

	newValues, err := f.GetRegisters(keys, f.tree.GetRoot().GetValue())
	if err != nil {
		t.Fatal(err)
	}
	for i, val := range newValues {
		if !bytes.Equal(values[i], val) {
			t.Fatalf("Something in GetRegister went wrong")
		}
	}

	f.CloseStorage()

}

func TestTrieUntrusted(t *testing.T) {
	f, err := NewTrieStorage(kvdbPath, tdbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll("./db/")

	keys, values := makeTestKeys()

	newRoot, _, err := f.UpdateRegistersWithProof(keys, values)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(f.tree.GetRoot().GetValue(), newRoot) {
		t.Fatalf("Something in UpdateRegister went wrong")
	}

	newValues, _, err := f.GetRegistersWithProof(keys, f.tree.GetRoot().GetValue())
	if err != nil {
		t.Fatal(err)
	}
	for i, val := range newValues {
		if !bytes.Equal(values[i], val) {
			t.Fatalf("Something in GetRegister went wrong")
		}
	}

	f.CloseStorage()

}

func makeTestKeys() ([][]byte, [][]byte) {
	key1 := make([]byte, 1)
	value1 := []byte{'a'}

	key2 := make([]byte, 1)
	value2 := []byte{'b'}
	utils.SetBit(key2, 5)

	key3 := make([]byte, 1)
	value3 := []byte{'c'}
	utils.SetBit(key3, 0)

	key4 := make([]byte, 1)
	value4 := []byte{'d'}
	utils.SetBit(key4, 0)
	utils.SetBit(key4, 5)

	keys := make([][]byte, 0)
	values := make([][]byte, 0)

	keys = append(keys, key1, key2, key3, key4)
	values = append(values, value1, value2, value3, value4)

	return keys, values
}
