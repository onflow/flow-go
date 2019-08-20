package crypto

type Severity int

const (
	// Warning means an error or panic might come later if this is not fixed
	Warning Severity = iota
	// Error means something was not right with the input or context but the
	// computation tried to continue.
	Error
	// Panis means the computation has stopped and couldn't continue.
	Panic
)

type cryptoError struct {
	msg string
	// TODO: This is commented for now as all kind of erros are encouraged to be fixed
	// but this might change
	//sev Severity
}

func (e cryptoError) Error() string {
	return e.msg
}
