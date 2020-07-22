package ledger

// ErrLedgerConstruction returned when there is a failure with ledger creation
type ErrLedgerConstruction struct {
	err error
}

func (e ErrLedgerConstruction) Error() string {
	return e.err.Error()
}

// Is return true if the type of errors are the same
func (e ErrLedgerConstruction) Is(other error) bool {
	_, ok := other.(ErrLedgerConstruction)
	return ok
}

func NewErrLedgerConstruction(err error) *ErrLedgerConstruction {
	return &ErrLedgerConstruction{err}
}

// ErrKeyNotFound returned when there are some missing keys in the ledger
type ErrKeyNotFound struct {
	missingKeys []Key
}

func (e ErrKeyNotFound) Error() string {
	str := "keys are missing: \n"
	for _, k := range e.missingKeys {
		str += "\t" + k.String() + "\n"
	}
	return str
}

// Is return true if the type of errors are the same
func (e ErrKeyNotFound) Is(other error) bool {
	_, ok := other.(ErrKeyNotFound)
	return ok
}

// TODO add more errors
