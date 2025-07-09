package scoring

import "github.com/onflow/flow-go/network/p2p"

// NoopInvCtrlMsgNotifConsumer is a no-op implementation of the p2p.GossipSubInvCtrlMsgNotifConsumer interface.
// It is used to consume invalid control message notifications from the GossipSub pubsub system and take no action.
// It is mainly used for cases when the peer scoring system is disabled.
type NoopInvCtrlMsgNotifConsumer struct {
}

func NewNoopInvCtrlMsgNotifConsumer() *NoopInvCtrlMsgNotifConsumer {
	return &NoopInvCtrlMsgNotifConsumer{}
}

var _ p2p.GossipSubInvCtrlMsgNotifConsumer = (*NoopInvCtrlMsgNotifConsumer)(nil)

func (n NoopInvCtrlMsgNotifConsumer) OnInvalidControlMessageNotification(_ *p2p.InvCtrlMsgNotif) {
	// no-op
}
