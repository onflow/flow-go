package util

import (
	"fmt"
	"net"
	"strconv"
	"strings"

	"github.com/dapperlabs/flow-go/protobuf/gossip/messages"
)

// Socket is a representation of IP addresses and ports that can be efficiently serialized into a compact form. Sockets can be used
// instead of sending string representations over the network

// NewSocket takes an IP address and a port and returns a socket that encapsulates this information
func NewSocket(address string) (*messages.Socket, error) {
	//extract ip and port from address
	ip, port, err := splitAddress(address)

	//invalid ip and port
	if err != nil {
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

// SocketToString extracts an address from a given socket
//TODO: IPV6 support
func SocketToString(s *messages.Socket) string {
	return fmt.Sprintf("%v:%v", net.IP(s.Ip), s.Port)
}
