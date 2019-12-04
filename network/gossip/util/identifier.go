package util

import (
	"encoding/hex"

	"github.com/dapperlabs/flow-go/model/flow"
)

func IDToString(id flow.Identifier) string {
	return hex.EncodeToString(id[:])
}

func StringToID(blob string) flow.Identifier {
	var id flow.Identifier
	_, _ = hex.Decode(id[:], []byte(blob))
	return id
}
