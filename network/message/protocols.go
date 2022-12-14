package message

type Protocol string

const (
	ProtocolPublish = Protocol("publish")
	ProtocolUnicast = Protocol("unicast")
)

func (p Protocol) String() string {
	return string(p)
}

// NewUnauthorizedProtocolError returns ErrUnauthorizedUnicastOnChannel or ErrUnauthorizedPublishOnChannel depending on the protocol provided.
func NewUnauthorizedProtocolError(p Protocol) error {
	if p == ProtocolUnicast {
		return ErrUnauthorizedUnicastOnChannel
	}

	return ErrUnauthorizedPublishOnChannel
}

type Protocols []Protocol

// Contains returns true if the protocol is in the list of Protocols.
func (pr Protocols) Contains(protocol Protocol) bool {
	for _, p := range pr {
		if p == protocol {
			return true
		}
	}

	return false
}
