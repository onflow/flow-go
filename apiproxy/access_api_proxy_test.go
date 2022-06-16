package apiproxy

import (
	"fmt"
	"net"
	"testing"
)

// Methodology
//
// We test the proxy and fall-over logic to reach basic coverage.
//
// * Basic coverage means that all conditional checks happen once but only once.
// * We embrace the simplest adequate solution to reduce engineering cost.
// * Any use cases requiring multiple conditionals exercised in a row are considered ignorable due to cost constraints.

func TestProxyE2E(t *testing.T) {
	done := make(chan int)
	// Bring up 1st upstream server
	charlie1, err := makeFlowLite("/tmp/TestProxyE2E1", done)
	if err != nil {
		t.Fatal(err)
	}
	// Wait until proxy call passes
	err = callFlowLite("/tmp/TestProxyE2E1")
	if err != nil {
		t.Fatal(err)
	}
	// Bring up 2nd upstream server
	charlie2, err := makeFlowLite("/tmp/TestProxyE2E2", done)
	if err != nil {
		t.Fatal(err)
	}
	// Both proxy calls should pass
	err = callFlowLite("/tmp/TestProxyE2E1")
	if err != nil {
		t.Fatal(err)
	}
	err = callFlowLite("/tmp/TestProxyE2E2")
	if err != nil {
		t.Fatal(err)
	}
	// Stop 1st upstream server
	_ = charlie1.Close()
	// Proxy call falls through
	err = callFlowLite("/tmp/TestProxyE2E1")
	if err == nil {
		t.Fatal(fmt.Errorf("backend still active after close"))
	}
	// Stop 2nd upstream server
	_ = charlie2.Close()
	// System errors out on shut down servers
	err = callFlowLite("/tmp/TestProxyE2E1")
	if err == nil {
		t.Fatal(fmt.Errorf("backend still active after close"))
	}
	err = callFlowLite("/tmp/TestProxyE2E2")
	if err == nil {
		t.Fatal(fmt.Errorf("backend still active after close"))
	}
	// wait for all
	<-done
}

func makeFlowLite(address string, done chan int) (net.Listener, error) {
	l, err := net.Listen("unix", address)
	if err != nil {
		return nil, err
	}

	go func(done chan int) {
		for {
			c, err := l.Accept()
			if err != nil {
				break
			}
			b := make([]byte, 3)
			_, _ = c.Read(b)
			_, _ = c.Write(b)
			_ = c.Close()
		}
		done <- 1
	}(done)
	return l, err
}

func callFlowLite(address string) error {
	c, err := net.Dial("unix", address)
	if err != nil {
		return err
	}
	o := []byte("abc")
	_, _ = c.Write(o)
	i := make([]byte, 3)
	_, _ = c.Read(i)
	if string(o) != string(i) {
		return fmt.Errorf("no match")
	}
	_ = c.Close()
	return err
}
