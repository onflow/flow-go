package gnode

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"strings"

	"github.com/dapperlabs/flow-go/pkg/grpc/shared"
	"github.com/dapperlabs/flow-go/pkg/network/gossip"
)

// ServePlacer is an interface for any protocol that can be used as a connection medium for gossip
type ServePlacer interface {
	Serve(net.Listener)
	Place(context.Context, string, *shared.GossipMessage, bool, gossip.Mode) (*shared.GossipReply, error)
}

// newSocket takes an IP address and a port and returns a socket that encapsulates this information
func newSocket(address string) (*shared.Socket, error) {
	//extract ip and port from address
	ip, port, err := splitAddress(address)

	//invalid ip and port
	if err != nil {
		fmt.Printf("invalid address")
		return &shared.Socket{}, fmt.Errorf("invalid address")
	}

	//parse to IP to bytes
	ipbytes := net.ParseIP(ip)

	if ipbytes == nil {
		fmt.Printf("invalid ip")
		return &shared.Socket{}, fmt.Errorf("invalid ip")
	}

	//parse the port
	portNum, err := strconv.Atoi(port)

	if portNum < 0 || err != nil {
		fmt.Printf("invalid port")
		return &shared.Socket{}, fmt.Errorf("invalid port")
	}

	return &shared.Socket{Ip: ipbytes, Port: uint32(portNum)}, nil
}

// split takes an address and splits it into ip and port
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

//TODO: IPV6 support
func socketToString(s *shared.Socket) string {
	return fmt.Sprintf("%v:%v", net.IP(s.Ip), s.Port)
}
