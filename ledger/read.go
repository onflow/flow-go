package ledger

// Read holds all data needed for a ledger read or ledger proof
type Read struct {
	StateCommitment StateCommitment
	Keys            []Key
}

// Size returns number of keys in the ledger read struct
func (r *Read) Size() int {
	return len(r.Keys)
}
