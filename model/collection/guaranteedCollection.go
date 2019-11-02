// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package collection

// GuaranteedCollection represents a signed hash for a collection, which is used
// to announce collections to consensus nodes.
type GuaranteedCollection struct {
	Hash      []byte
	Signature []byte
}
