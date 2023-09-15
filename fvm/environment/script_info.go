package environment

import (
	"github.com/onflow/flow-go/model/fingerprint"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/hash"
)

type ScriptInfoParams struct {
	ID        flow.Identifier
	Script    []byte
	Arguments [][]byte
}

func (info ScriptInfoParams) Fingerprint() []byte {
	return fingerprint.Fingerprint(struct {
		Script    []byte
		Arguments [][]byte
	}{
		Script:    info.Script,
		Arguments: info.Arguments,
	})
}

func NewScriptInfoParams(code []byte, arguments [][]byte) *ScriptInfoParams {
	info := &ScriptInfoParams{
		Script:    code,
		Arguments: arguments,
	}
	info.ID = flow.HashToID(hash.DefaultComputeHash(info.Fingerprint()))
	return info
}
