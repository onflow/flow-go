package ledger

// Update holds all data needed for a ledger update
type Update struct {
	StateCommitment StateCommitment
	Keys            []Key
	Values          []Value
}

// Size returns number of keys in the ledger update
func (u *Update) Size() int {
	return len(u.Keys)
}

// TrieUpdate holds all data for a trie update
type TrieUpdate struct {
	StateCommitment StateCommitment
	Paths           []Path
	Payloads        []*Payload
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
	if !u.StateCommitment.Equals(other.StateCommitment) {
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
