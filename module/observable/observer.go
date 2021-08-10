package observable

type Observer interface {
	// conveys an item that is emitted by the Observable to the observer
	OnNext(interface{})
	// indicates that the Observable has terminated with a specified error condition and that it will be emitting no further items
	OnError(err error)
	// indicates that the Observable has completed successfully and that it will be emitting no further items
	OnComplete()
}
