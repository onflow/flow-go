package ledger

import (
	"bytes"
	"os"
	"testing"
)

func TestTrieUntrustedAndVerify(t *testing.T) {
	f, err := NewTrieStorage()
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll("./db/")

	keys, values := makeTestKeys()

	newRoot, proofs, err := f.UpdateRegistersWithProof(keys, values)

	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(f.tree.GetRoot().GetValue(), newRoot) {
		t.Fatalf("Something in UpdateRegister went wrong")
	}

	v := NewTrieVerifier(f.tree.GetHeight(), f.tree.GetDefaultHashes())
	_, err = v.VerifyRegistersProof(keys, f.tree.GetRoot().GetValue(), values, proofs)
	if err != nil {
		t.Fatal(err)
	}

	f.CloseStorage()
}
