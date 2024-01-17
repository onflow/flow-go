// Package pathfinder computes the trie storage path for any given key/value pair
package pathfinder

import (
	"crypto/sha256"
	"fmt"

	"github.com/onflow/crypto/hash"

	"github.com/onflow/flow-go/ledger"
)

// PathByteSize captures number of bytes each path takes
const PathByteSize = 32

// KeyToPath converts key into a path
// version zero applies sha2-256 on value of the key parts (in order ignoring types)
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
				return ledger.DummyPath, err
			}
			path, err := ledger.ToPath(h.Sum(nil))
			if err != nil {
				return ledger.DummyPath, err
			}
			return path, nil
		}
	case 1:
		{
			var path ledger.Path
			hash.ComputeSHA3_256((*[ledger.PathLen]byte)(&path), key.CanonicalForm())
			return path, nil
		}
	}
	return ledger.DummyPath, fmt.Errorf("unsupported key to path version")
}

// KeysToPaths converts an slice of keys into a paths
func KeysToPaths(keys []ledger.Key, version uint8) ([]ledger.Path, error) {
	paths := make([]ledger.Path, len(keys))
	for i, k := range keys {
		p, err := KeyToPath(k, version)
		if err != nil {
			return nil, err
		}
		paths[i] = p
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

	trieUpdate := &ledger.TrieUpdate{RootHash: ledger.RootHash(u.State()), Paths: paths, Payloads: payloads}

	return trieUpdate, nil
}

// QueryToTrieRead converts a ledger query into a trie read
func QueryToTrieRead(q *ledger.Query, version uint8) (*ledger.TrieRead, error) {

	paths, err := KeysToPaths(q.Keys(), version)
	if err != nil {
		return nil, err
	}

	trieRead := &ledger.TrieRead{RootHash: ledger.RootHash(q.State()), Paths: paths}

	return trieRead, nil
}

// PayloadsToValues extracts values from an slice of payload
func PayloadsToValues(payloads []*ledger.Payload) ([]ledger.Value, error) {
	ret := make([]ledger.Value, len(payloads))
	for i, p := range payloads {
		ret[i] = p.Value()
	}
	return ret, nil
}

// PathsFromPayloads constructs paths from an slice of payload
func PathsFromPayloads(payloads []*ledger.Payload, version uint8) ([]ledger.Path, error) {
	paths := make([]ledger.Path, len(payloads))
	for i, pay := range payloads {
		k, err := pay.Key()
		if err != nil {
			return nil, err
		}
		p, err := KeyToPath(k, version)
		if err != nil {
			return nil, err
		}
		paths[i] = p
	}
	return paths, nil
}

// UpdateToPayloads constructs an slice of payloads given ledger update
func UpdateToPayloads(update *ledger.Update) ([]*ledger.Payload, error) {
	keys := update.Keys()
	values := update.Values()
	payloads := make([]*ledger.Payload, len(keys))
	for i := range keys {
		payload := ledger.NewPayload(keys[i], values[i])
		payloads[i] = payload
	}
	return payloads, nil
}
