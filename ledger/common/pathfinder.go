package common

import (
	"crypto/sha256"
	"fmt"

	"github.com/dapperlabs/flow-go/ledger"
)

// KeyToPath converts key into a path
func KeyToPath(key ledger.Key, version uint8) (ledger.Path, error) {
	switch version {
	case 0:
		{
			ret := make([]byte, 0)
			for _, kp := range key.KeyParts {
				ret = append(ret, kp.Value...)
			}
			h := sha256.New()
			_, err := h.Write(ret)
			if err != nil {
				return nil, err
			}
			return ledger.Path(h.Sum(nil)), nil
		}
	}
	return nil, fmt.Errorf("unsuported key to path version")
}

// KeysToPaths converts an slice of keys into a paths
func KeysToPaths(keys []ledger.Key, version uint8) ([]ledger.Path, error) {
	paths := make([]ledger.Path, 0)
	for _, k := range keys {
		p, err := KeyToPath(k, version)
		if err != nil {
			return nil, err
		}
		paths = append(paths, p)
	}
	return paths, nil
}

// UpdateToTrieUpdate converts an update into a trie update
func UpdateToTrieUpdate(u *ledger.Update, version uint8) (*ledger.TrieUpdate, error) {

	paths, err := KeysToPaths(u.Keys(), version)
	if err != nil {
		return nil, err
	}

	payloads, err := UpdateToPayloads(u)
	if err != nil {
		return nil, err
	}

	trieUpdate := &ledger.TrieUpdate{RootHash: ledger.RootHash(u.StateCommitment()), Paths: paths, Payloads: payloads}

	return trieUpdate, nil
}

// QueryToTrieRead converts a ledger query into a trie read
func QueryToTrieRead(q *ledger.Query, version uint8) (*ledger.TrieRead, error) {

	paths, err := KeysToPaths(q.Keys(), version)
	if err != nil {
		return nil, err
	}

	trieRead := &ledger.TrieRead{RootHash: ledger.RootHash(q.StateCommitment()), Paths: paths}

	return trieRead, nil
}
