package crypto

type cryptoError struct {
	msg string
}

func (e cryptoError) Error() string {
	return e.msg
}
