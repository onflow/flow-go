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
	ret := make([]ledger.Value, 0)
	for _, p := range payloads {
		ret = append(ret, p.Value)
	}
	return ret, nil
}

// UpdateToPayloads constructs an slice of payloads given ledger update
func UpdateToPayloads(update *ledger.Update) ([]*ledger.Payload, error) {
	keys := update.Keys()
	values := update.Values()
	payloads := make([]*ledger.Payload, 0)
	for i := range keys {
		payload := &ledger.Payload{Key: keys[i], Value: values[i]}
		payloads = append(payloads, payload)
	}
	return payloads, nil
}
