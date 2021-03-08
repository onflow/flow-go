package common

import (
	"flag"
	"testing"
)

func TestNodeIDsArgs(t *testing.T) {
	var flags flag.FlagSet
	flags.Init("test", flag.ContinueOnError)
	var v NodeIDsArgs
	flags.Var(&v, "v", "usage")
	if err := flags.Parse([]string{"-v", "1", "-v", "2", "-v=3"}); err != nil {
		t.Error(err)
	}
	if len(v) != 3 {
		t.Fatal("expected 3 args; got ", len(v))
	}
	expect := "[1 2 3]"
	if v.String() != expect {
		t.Errorf("expected value %q got %q", expect, v.String())
	}
}
