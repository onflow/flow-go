package ledger

import (
	"bytes"
	"encoding/hex"
	"fmt"
)

// TrieRead captures a trie read query
type TrieRead struct {
	RootHash RootHash
	Paths    []Path
}

// TrieUpdate holds all data for a trie update
type TrieUpdate struct {
	RootHash RootHash
	Paths    []Path
	Payloads []*Payload
}

// Size returns number of paths in the trie update
func (u *TrieUpdate) Size() int {
	return len(u.Paths)
}

// IsEmpty returns true if key or value is not empty
func (u *TrieUpdate) IsEmpty() bool {
	return u.Size() == 0
}

// Equals compares this trie update to another trie update
func (u *TrieUpdate) Equals(other *TrieUpdate) bool {
	if other == nil {
		return false
	}
	if !u.RootHash.Equals(other.RootHash) {
		return false
	}
	if len(u.Paths) != len(other.Paths) {
		return false
	}
	for i := range u.Paths {
		if !u.Paths[i].Equals(other.Paths[i]) {
			return false
		}
	}

	if len(u.Payloads) != len(other.Payloads) {
		return false
	}
	for i := range u.Payloads {
		if !u.Payloads[i].Equals(other.Payloads[i]) {
			return false
		}
	}
	return true
}

// RootHash captures the root hash of a trie
type RootHash []byte

func (rh RootHash) String() string {
	return hex.EncodeToString(rh)
}

// Equals compares the root hash to another one
func (rh RootHash) Equals(o RootHash) bool {
	if o == nil {
		return false
	}
	return bytes.Equal(rh, o)
}

// Path captures storage path of a payload;
// where we store a payload in the ledger
type Path []byte

func (p Path) String() string {
	str := ""
	for _, i := range p {
		str += fmt.Sprintf("%08b", i)
	}
	if len(str) > 16 {
		str = str[0:8] + "..." + str[len(str)-8:]
	}
	return str
}

// Size returns the size of the path
func (p *Path) Size() int {
	return len(*p)
}

// Equals compares this path to another path
func (p Path) Equals(o Path) bool {
	if o == nil {
		return false
	}
	return bytes.Equal([]byte(p), []byte(o))
}

// Payload is the smallest immutable storable unit in ledger
type Payload struct {
	Key   Key
	Value Value
}

// Size returns the size of the payload
func (p *Payload) Size() int {
	return p.Key.Size() + p.Value.Size()
}

// IsEmpty returns true if key or value is not empty
func (p *Payload) IsEmpty() bool {
	return p.Size() == 0
}

// TODO fix me
func (p *Payload) String() string {
	// TODO improve this key, values
	return p.Key.String() + " " + p.Value.String()
}

// Equals compares this payload to another payload
func (p *Payload) Equals(other *Payload) bool {
	if other == nil {
		return false
	}
	if !p.Key.Equals(&other.Key) {
		return false
	}
	if !p.Value.Equals(other.Value) {
		return false
	}
	return true
}

// NewPayload returns a new payload
func NewPayload(key Key, value Value) *Payload {
	return &Payload{Key: key, Value: value}
}

// EmptyPayload returns an empty payload
func EmptyPayload() *Payload {
	return &Payload{}
}
