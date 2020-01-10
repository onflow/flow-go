// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package logging

import (
	"encoding/hex"

	"github.com/dapperlabs/flow-go/model/flow"
)

func ID(entity flow.Entity) []byte {
	id := entity.ID()
	return id[:]
}

func IDs(ids []flow.Identifier) []string {
	ss := make([]string, 0, len(ids))
	for _, id := range ids {
		ss = append(ss, hex.EncodeToString(id[:]))
	}
	return ss
}
