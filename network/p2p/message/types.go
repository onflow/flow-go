package p2pmsg

// ControlMessageType is the type of control message, as defined in the libp2p pubsub spec.
type ControlMessageType string

func (c ControlMessageType) String() string {
	return string(c)
}

const (
	CtrlMsgRPC        ControlMessageType = "RPC"
	CtrlMsgIHave      ControlMessageType = "IHAVE"
	CtrlMsgIWant      ControlMessageType = "IWANT"
	CtrlMsgGraft      ControlMessageType = "GRAFT"
	CtrlMsgPrune      ControlMessageType = "PRUNE"
	RpcPublishMessage ControlMessageType = "RpcPublishMessage"
)

// ControlMessageTypes returns list of all libp2p control message types.
func ControlMessageTypes() []ControlMessageType {
	return []ControlMessageType{CtrlMsgIHave, CtrlMsgIWant, CtrlMsgGraft, CtrlMsgPrune}
}
