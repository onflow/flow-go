package mocks

// MockHub is a value that stores mocked networks in order for them to send events directly
type MockHub struct {
	Networks map[string]*MockNetwork
}

// NewNetworkHub returns a MockHub value with empty network slice
func NewNetworkHub() *MockHub {
	return &MockHub{
		Networks: make(map[string]*MockNetwork),
	}
}

// Plug stores the reference of the network in the hub object, in order for networks to find
// other network to send events directly
func (hub *MockHub) Plug(net *MockNetwork) {
	hub.Networks[net.GetID()] = net
}
