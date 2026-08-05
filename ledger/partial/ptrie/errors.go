package ptrie

import "github.com/onflow/flow-go/ledger"

type ErrMissingPath struct {
	Paths []ledger.Path
}

func (e ErrMissingPath) Error() string {
	str := "paths are missing: \n"
	for _, k := range e.Paths {
		str += "\t" + k.String() + "\n"
	}
	return str
}
