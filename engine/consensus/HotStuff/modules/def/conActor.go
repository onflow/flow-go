package def

type ConActor interface {
	Vote(*Block)
	SendViewChange()
	PassMsgToConActor(interface{})
}
