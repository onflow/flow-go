package gossip

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"strings"

	"github.com/dapperlabs/flow-go/proto/gossip/messages"
)

// ServePlacer is an interface for any protocol that can be used as a connection medium for gossip
type ServePlacer interface {
	Serve(net.Listener)
	Place(context.Context, string, *messages.GossipMessage, bool, Mode) (*messages.GossipReply, error)
}

// newSocket takes an IP address and a port and returns a socket that encapsulates this information
func newSocket(address string) (*messages.Socket, error) {
	//extract ip and port from address
	ip, port, err := splitAddress(address)

	//invalid ip and port
	if err != nil {
		fmt.Printf("invalid address")
		return &messages.Socket{}, fmt.Errorf("invalid address")
	}

	//parse to IP to bytes
	ipbytes := net.ParseIP(ip)

	if ipbytes == nil {
		return &messages.Socket{}, fmt.Errorf("invalid ip")
	}

	//parse the port
	portNum, err := strconv.Atoi(port)

	if portNum < 0 || err != nil {
		return &messages.Socket{}, fmt.Errorf("invalid port")
	}

	return &messages.Socket{Ip: ipbytes, Port: uint32(portNum)}, nil
}

// splitAddress takes an address and splits it into ip and port
func splitAddress(address string) (string, string, error) {
	//index of the last colon in the address
	colonPos := strings.LastIndexByte(address, ':')
	if colonPos == -1 {
		return "", "", fmt.Errorf("invalid address, could not find colon")
	}

	ip := address[:colonPos]
	port := address[colonPos+1:]

	return ip, port, nil
}

// socketToString extracts an address from a given socket
//TODO: IPV6 support
func socketToString(s *messages.Socket) string {
	return fmt.Sprintf("%v:%v", net.IP(s.Ip), s.Port)
}
