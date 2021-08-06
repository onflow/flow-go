package observable

type Observable interface {
	// indicates that the observer is ready to receive notifications from the Observable
	Subscribe(observer Observer)
	// indicates that the observer no longer wants to receive notifications from the Observable
	Unsubscribe(observer Observer)
}
