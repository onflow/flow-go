package util

import (
	"bytes"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
)

func TestLogProgress(t *testing.T) {
	buf := bytes.NewBufferString("")
	lg := zerolog.New(buf)
	logger := LogProgress("test", 40, &lg)
	for i := 0; i < 50; i++ {
		logger(i)
	}

	expectedLogs :=
		`{"level":"info","message":"test completion percentage: 10 percent"}
{"level":"info","message":"test completion percentage: 20 percent"}
{"level":"info","message":"test completion percentage: 30 percent"}
{"level":"info","message":"test completion percentage: 40 percent"}
{"level":"info","message":"test completion percentage: 50 percent"}
{"level":"info","message":"test completion percentage: 60 percent"}
{"level":"info","message":"test completion percentage: 70 percent"}
{"level":"info","message":"test completion percentage: 80 percent"}
{"level":"info","message":"test completion percentage: 90 percent"}
{"level":"info","message":"test completion percentage: 100 percent"}
{"level":"info","message":"test completion percentage: 110 percent"}
{"level":"info","message":"test completion percentage: 120 percent"}
`
	require.Equal(t, expectedLogs, buf.String())
}
