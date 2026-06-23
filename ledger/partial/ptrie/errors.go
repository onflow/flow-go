package ptrie

import (
	"strings"

	"github.com/onflow/flow-go/ledger"
)

type ErrMissingPath struct {
	Paths []ledger.Path
}

func (e ErrMissingPath) Error() string {
	var str strings.Builder
	str.WriteString("paths are missing: \n")
	for _, k := range e.Paths {
		str.WriteString("\t" + k.String() + "\n")
	}
	return str.String()
}
