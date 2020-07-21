package ledger

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
