package logging

const (
	// KeySuspicious is a logging label. It flags that the log event informs about suspected BYZANTINE
	// behaviour observed by this node. Adding such labels is beneficial for easily searching log events
	// regarding misbehaving nodes.
	// Axiomatically, each node considers its own operator as well as Flow's governance committee as trusted.
	// Hence, potential problems with inputs (mostly configurations) from these sources are *not* considered
	// byzantine. To flag inputs from these sources, please use [logging.KeyPotentialConfigurationProblem].
	KeySuspicious = "suspicious"

	// KeyPotentialConfigurationProblem is a logging label. It flags that the log event informs about suspected
	// configuration problem by this node. Adding such labels is beneficial for easily searching log events
	// regarding potential configuration problems.
	KeyPotentialConfigurationProblem = "potential-configuration-problem"

	// KeyNetworkingSecurity is a logging label that is used to flag the log event as a networking security issue.
	// This is used to add an easily searchable label to the log events.
	KeyNetworkingSecurity = "networking-security"

	// KeyProtocolViolation is a logging label that is used to flag the log event as byzantine protocol violation.
	// This is used to add an easily searchable label to the log events.
	KeyProtocolViolation = "byzantine-protocol-violation"

	// KeyLoad is a logging label that is used to flag the log event as a load issue.
	KeyLoad = "load"
)
