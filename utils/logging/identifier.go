package logging

import (
	"encoding/hex"
	"fmt"

	"github.com/onflow/flow-go/model/flow"
)

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
