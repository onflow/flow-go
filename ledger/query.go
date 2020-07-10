package ledger

// Query holds all data needed for a ledger read or ledger proof
type Query struct {
	StateCommitment StateCommitment
	Keys            []Key
}

// Size returns number of keys in the query
func (q *Query) Size() int {
	return len(q.Keys)
}
