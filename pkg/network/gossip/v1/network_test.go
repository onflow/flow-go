package gnode

import (
	"fmt"
	"net"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewSocket(t *testing.T) {
	assert := assert.New(t)

	tt := []struct {
		address string
		ip      []byte
		port    uint32
		err     error
	}{
		{ //ipv4 address
			address: "192.168.1.1:1099",
			ip:      []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 255, 255, 192, 168, 1, 1}, //ipv6 bytes
			port:    1099,
			err:     nil,
		},
		{ //no port
			address: "192.168.1.1",
			err:     fmt.Errorf("non nil"),
		},
		{ //no address
			address: ":1099",
			err:     fmt.Errorf("non nil"),
		},
		{ //empty string
			address: "",
			err:     fmt.Errorf("non nil"),
		},
		{ //ipv6 address
			address: "0:0:0:0:0:ffff:c0a8:101:1099",
			ip:      []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 255, 255, 192, 168, 1, 1}, //ipv6 bytes
			port:    1099,
			err:     nil,
		},
		{ //ipv6 address with ellipsis
			address: "::ffff:c0a8:101:1099",
			ip:      []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 255, 255, 192, 168, 1, 1}, //ipv6 bytes
			port:    1099,
			err:     nil,
		},
	}

	for _, tc := range tt {
		socket, err := newSocket(tc.address)
		if tc.err == nil {
			assert.Nil(err)
		}

		if tc.err != nil {
			assert.NotNil(err)
		}

		if err != nil && tc.err != nil {
			continue
		}

		if !reflect.DeepEqual(tc.ip, socket.Ip) {
			t.Log([]byte(net.ParseIP("192.168.1.1")))
			t.Log(socketToString(socket))
			t.Errorf("address mismatch. expected: %v, got: %v", tc.ip, socket.Ip)
		}
		assert.Equal(tc.port, socket.Port)
	}
}

func TestSplit(t *testing.T) {
	assert := assert.New(t)

	tt := []struct {
		address string
		ip      string
		port    string
		err     error
	}{
		{ //ipv4 address
			address: "192.168.1.1:1099",
			ip:      "192.168.1.1", //ipv6 bytes
			port:    "1099",
			err:     nil,
		},
		{ //no port
			address: "192.168.1.1",
			err:     fmt.Errorf("non nil"),
		},
		{ //no address
			address: "1099",
			err:     fmt.Errorf("non nil"),
		},
		{ //empty string
			address: "",
			err:     fmt.Errorf("non nil"),
		},
		{ //ipv6 address
			address: "0:0:0:0:0:ffff:c0a8:101:1099",
			ip:      "0:0:0:0:0:ffff:c0a8:101", //ipv6 bytes
			port:    "1099",
			err:     nil,
		},
		{ //ipv6 address with ellipsis
			address: "::ffff:c0a8:101:1099",
			ip:      "::ffff:c0a8:101", //ipv6 bytes
			port:    "1099",
			err:     nil,
		},
	}

	for _, tc := range tt {
		ip, port, err := splitAddress(tc.address)
		if tc.err == nil {
			assert.Nil(err)
		}

		if tc.err != nil {
			assert.NotNil(err)
		}

		if err != nil && tc.err != nil {
			continue
		}

		if err == nil && tc.err == nil {
			assert.Equal(tc.ip, ip)
			assert.Equal(tc.port, port)
		}
	}
}

// Testing whether to ip returns the same IP we used to create the socket or not. In case of ipv6
// it returns the shortest IP which maps to the same address
func TestToString(t *testing.T) {
	assert := assert.New(t)

	tt := []struct {
		address  string
		expected string
	}{
		{ //ipv4
			address:  "192.169.10.1:1099",
			expected: "192.169.10.1:1099",
		},
		{ //ipv6 which can be shortened
			address:  "1:0:0:0:0:ffff:c0a8:101:1099",
			expected: "1::ffff:c0a8:101:1099",
		},
		{ //ipv6 which is already shortened
			address:  "1::ffff:c0a8:101:1099",
			expected: "1::ffff:c0a8:101:1099",
		},
		{ //ipv6 which cannot be shortened
			address:  "1:ff:ff:101:99:ffff:c0a8:101:1099",
			expected: "1:ff:ff:101:99:ffff:c0a8:101:1099",
		},
	}

	for _, tc := range tt {
		socket, err := newSocket(tc.address)
		assert.Nil(err)
		socketString := socketToString(socket)
		assert.Equal(tc.expected, socketString)
	}
}
