// Package pathfinder computes the trie storage path for any given key/value pair
package pathfinder

import (
	"crypto/sha256"
	"fmt"

	"github.com/onflow/flow-go/crypto/hash"
	"github.com/onflow/flow-go/ledger"
)

// PathByteSize captures number of bytes each path takes
const PathByteSize = 32

// KeyIDToPath converts key into a path
// version zero applies sha2-256 on value of the key parts (in order ignoring types)
// WARNING: KeyIDToPath() and KeyIDToKey(k).KeyToPath() must
// produce the same path. Changes to these functions must be in sync.
func KeyIDToPath(keyID ledger.KeyID, version uint8) (ledger.Path, error) {
	switch version {
	case 0:
		{
			ret := make([]byte, 0)
			ret = append(ret, keyID.Owner...)
			ret = append(ret, keyID.Controller...)
			ret = append(ret, keyID.Key...)
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
			hash.ComputeSHA3_256((*[ledger.PathLen]byte)(&path), keyID.CanonicalForm())
			return path, nil
		}
	}
	return ledger.DummyPath, fmt.Errorf("unsupported key to path version")
}

// KeysToPaths converts an slice of keyIDs into paths.
func KeyIDsToPaths(keyIDs []ledger.KeyID, version uint8) ([]ledger.Path, error) {
	paths := make([]ledger.Path, len(keyIDs))
	for i, k := range keyIDs {
		p, err := KeyIDToPath(k, version)
		if err != nil {
			return nil, err
		}
		paths[i] = p
	}
	return paths, nil
}

// KeyToPath converts key into a path
// version zero applies sha2-256 on value of the key parts (in order ignoring types)
// WARNING: KeyIDToPath() and KeyIDToKey(k).KeyToPath() must
// produce the same path. Changes to these functions must be in sync.
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

// PayloadsToValues extracts values from an slice of payload
func PayloadsToValues(payloads []*ledger.Payload) ([]ledger.Value, error) {
	ret := make([]ledger.Value, len(payloads))
	for i, p := range payloads {
		ret[i] = p.Value
	}
	return ret, nil
}

// PathsFromPayloads constructs paths from an slice of payload
func PathsFromPayloads(payloads []ledger.Payload, version uint8) ([]ledger.Path, error) {
	paths := make([]ledger.Path, len(payloads))
	for i, pay := range payloads {
		p, err := KeyToPath(pay.Key, version)
		if err != nil {
			return nil, err
		}
		paths[i] = p
	}
	return paths, nil
}
