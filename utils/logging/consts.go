package logging

const (
	// KeySuspicious is a logging label that is used to flag the log event as suspicious behavior
	// This is used to add an easily searchable label to the log event
	KeySuspicious = "suspicious"

	// KeyNetworkingSecurity is a logging label that is used to flag the log event as a networking security issue.
	// This is used to add an easily searchable label to the log events.
	KeyNetworkingSecurity = "networking-security"

	// KeyLoad is a logging label that is used to flag the log event as a load issue.
	KeyLoad = "load"
)
