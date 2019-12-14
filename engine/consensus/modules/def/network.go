package def

type Network interface {
	Send([]string, interface{})
}
