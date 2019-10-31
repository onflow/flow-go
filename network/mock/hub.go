package mock

// Hub is a value that stores mocked networks in order for them to send events directly
type Hub struct {
	Networks map[string]*Network
}

// NewNetworkHub returns a MockHub value with empty network slice
func NewNetworkHub() *Hub {
	return &Hub{
		Networks: make(map[string]*Network),
	}
}

// Plug stores the reference of the network in the hub object, in order for networks to find
// other network to send events directly
func (hub *Hub) Plug(net *Network) {
	hub.Networks[net.GetID()] = net
}
