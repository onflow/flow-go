// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package operation

import (
	"encoding/binary"

	"github.com/dgraph-io/badger/v2"

	"github.com/dapperlabs/flow-go/model/flow"
)

func toDeltaKey(number uint64, role flow.Role, nodeID flow.Identifier) []byte {
	key := make([]byte, 42)
	key[0] = codeDelta
	binary.BigEndian.PutUint64(key[1:9], number)
	key[9] = uint8(role)
	copy(key[10:42], nodeID[:])
	return key
}

func fromDeltaKey(key []byte) (uint64, flow.Role, flow.Identifier) {
	number := binary.BigEndian.Uint64(key[1:9])
	role := flow.Role(key[9])
	var nodeID flow.Identifier
	copy(nodeID[:], key[10:42])
	return number, role, nodeID
}

func InsertDelta(number uint64, role flow.Role, nodeID flow.Identifier, delta int64) func(*badger.Txn) error {
	return insert(toDeltaKey(number, role, nodeID), delta)
}

func RetrieveDelta(number uint64, role flow.Role, nodeID flow.Identifier, delta *int64) func(*badger.Txn) error {
	return retrieve(toDeltaKey(number, role, nodeID), delta)
}

func TraverseDeltas(from uint64, to uint64, filters []flow.IdentityFilter, process func(number uint64, role flow.Role, nodeID flow.Identifier, delta int64) error) func(*badger.Txn) error {
	iteration := func() (checkFunc, createFunc, handleFunc) {
		var number uint64
		var role flow.Role
		var nodeID flow.Identifier
		var delta int64
		check := func(key []byte) bool {
			number, role, nodeID = fromDeltaKey(key)
			id := flow.Identity{NodeID: nodeID, Role: role}
			for _, filter := range filters {
				if !filter(&id) {
					return false
				}
			}
			return true
		}
		create := func() interface{} {
			return &delta
		}
		handle := func() error {
			return process(number, role, nodeID, delta)
		}
		return check, create, handle
	}
	return iterate(makePrefix(codeDelta, from), makePrefix(codeDelta, to+1), iteration)
}
