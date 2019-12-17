package viewChangeHandlerEvents

import "github.com/dapperlabs/flow-go/engine/consensus/HotStuff/modules/def"

// Processor consumes events produced by viewChanger
// Implementation must be concurrency safe; Non-blocking;
// and must handle repetition of the same events (with some processing overhead).
type Processor interface {
	OnSentViewChange(*def.QuorumCertificate)
	OnReceivedViewChange(*def.QuorumCertificate)
	OnViewChangeReceived(*def.QuorumCertificate)
}

type SentViewChangeConsumer interface {
	OnSentViewChange(*def.QuorumCertificate)
}

type ReceivedViewChangeConsumer interface {
	OnReceivedViewChange(*def.QuorumCertificate)
}
