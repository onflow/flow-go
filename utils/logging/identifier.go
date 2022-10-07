// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package logging

import (
	"encoding/hex"
	"fmt"

	"github.com/onflow/flow-go/model/flow"
)

// KeySuspicious is a logging label that is used to flag the log event as suspicious behavior
// This is used to add an easily searchable label to the log event
const KeySuspicious = "suspicious"

func Entity(entity flow.Entity) []byte {
	id := entity.ID()
	return id[:]
}

func ID(id flow.Identifier) []byte {
	return id[:]
}

func Type(obj interface{}) string {
	return fmt.Sprintf("%T", obj)
}

func IDs(ids []flow.Identifier) []string {
	ss := make([]string, 0, len(ids))
	for _, id := range ids {
		ss = append(ss, hex.EncodeToString(id[:]))
	}
	return ss
}
