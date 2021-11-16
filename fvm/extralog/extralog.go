package extralog

import (
	"fmt"
	"io/ioutil"
)

// ExtraLogDumpPath is a shared variable with path where to dump extra logs
// Currently we dump them inside Transaction structure and due to temporary nature of this fix
// it's easier to use global variable rather than pass extra parameters down multiple levels,
// changing multiple tests etc.
var ExtraLogDumpPath string

func init() {
	dir, err := ioutil.TempDir("", "extra-log-dumps")
	if err != nil {
		panic(fmt.Errorf("cannot create temp dir for extra logs dumps: %w", err))
	}
	ExtraLogDumpPath = dir
}
