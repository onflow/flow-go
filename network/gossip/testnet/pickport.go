package testnet
import (
	"fmt"
	"net"
)

// PickPort picks and returns the first available port from port pool
// PickPort solely is used for our testing purposes
func PickPort(portPool []string) (net.Listener, error) {
	for _, port := range portPool {
		ln, err := net.Listen("tcp4", port)
		if err == nil {
			return ln, nil
		}
	}
	return nil, fmt.Errorf("could not find an empty port in the given pool")
}
