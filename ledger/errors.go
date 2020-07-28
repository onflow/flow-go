package ledger

// ErrLedgerConstruction returned when there is a failure with ledger creation
type ErrLedgerConstruction struct {
	Err error
}

func (e ErrLedgerConstruction) Error() string {
	return e.Err.Error()
}

// Is return true if the type of errors are the same
func (e ErrLedgerConstruction) Is(other error) bool {
	_, ok := other.(ErrLedgerConstruction)
	return ok
}

// NewErrLedgerConstruction constructs a new ledger construction error
func NewErrLedgerConstruction(err error) *ErrLedgerConstruction {
	return &ErrLedgerConstruction{err}
}

// ErrMissingKeys returned when there are some missing keys in the ledger
type ErrMissingKeys struct {
	Keys []Key
}

func (e ErrMissingKeys) Error() string {
	str := "keys are missing: \n"
	for _, k := range e.Keys {
		str += "\t" + k.String() + "\n"
	}
	return str
}

// Is return true if the type of errors are the same
func (e ErrMissingKeys) Is(other error) bool {
	_, ok := other.(ErrMissingKeys)
	return ok
}

// TODO add more errors
// ErrorFetchQuery
// ErrorCommitChanges
// ErrorMissingKeys
