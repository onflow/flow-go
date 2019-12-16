package ingest

type ErrIncompleteTransaction struct{}

// TODO
func (e ErrIncompleteTransaction) Error() string {
	return "incomplete"
}
