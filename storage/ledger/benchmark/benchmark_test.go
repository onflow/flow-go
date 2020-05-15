package main

import (
	"testing"
)

func TestBenchamark(t *testing.T) {
	t.Skipf("manual debug only")
	StorageBenchmark()
}
