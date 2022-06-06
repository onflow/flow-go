package ledger

// ErrLedgerConstruction is returned upon a failure in ledger creation steps
type ErrLedgerConstruction struct {
	Err error
}

func (e ErrLedgerConstruction) Error() string {
	return e.Err.Error()
}

// Is returns true if the type of errors are the same
func (e ErrLedgerConstruction) Is(other error) bool {
	_, ok := other.(ErrLedgerConstruction)
	return ok
}

// NewErrLedgerConstruction constructs a new ledger construction error
func NewErrLedgerConstruction(err error) *ErrLedgerConstruction {
	return &ErrLedgerConstruction{err}
}

// ErrMissingPaths is returned when some paths are not found in the ledger
// this is mostly used when dealing with partial ledger
type ErrMissingPaths struct {
	Paths []Path
}

func (e ErrMissingPaths) Error() string {
	str := "paths are missing: \n"
	for _, p := range e.Paths {
		str += "\t" + p.String() + "\n"
	}
	return str
}

// Is returns true if the type of errors are the same
func (e ErrMissingPaths) Is(other error) bool {
	_, ok := other.(ErrMissingPaths)
	return ok
}

// TODO add more errors
// ErrorFetchQuery
// ErrorCommitChanges
// ErrorMissingKeys
