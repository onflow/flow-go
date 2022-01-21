package cmd

import (
	"fmt"
	"sync"
)

type testLog struct {
	logs []string
	mux  sync.Mutex
}

// handle concurrent logging
func (l *testLog) Logf(msg string, args ...interface{}) {
	l.Log(fmt.Sprintf(msg, args...))
}

func (l *testLog) Log(msg string) {
	l.mux.Lock()
	defer l.mux.Unlock()
	l.logs = append(l.logs, msg)
}

func (l *testLog) Reset() {
	l.logs = []string{}
}
