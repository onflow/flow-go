package util

import (
	"encoding/hex"

	"github.com/dapperlabs/flow-go/model"
)

func IDToString(id model.Identifier) string {
	return hex.EncodeToString(id[:])
}

func StringToID(blob string) model.Identifier {
	var id model.Identifier
	_, _ = hex.Decode(id[:], []byte(blob))
	return id
}
