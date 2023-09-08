package util

import (
	"bytes"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
)

func TestLogProgress40(t *testing.T) {
	buf := bytes.NewBufferString("")
	lg := zerolog.New(buf)
	total := 40
	logger := LogProgress("test", total, lg)
	for i := 0; i < total; i++ {
		logger(i)
	}

	expectedLogs :=
		`{"level":"info","message":"test progress: 0 percent"}
{"level":"info","message":"test progress: 10 percent"}
{"level":"info","message":"test progress: 20 percent"}
{"level":"info","message":"test progress: 30 percent"}
{"level":"info","message":"test progress: 40 percent"}
{"level":"info","message":"test progress: 50 percent"}
{"level":"info","message":"test progress: 60 percent"}
{"level":"info","message":"test progress: 70 percent"}
{"level":"info","message":"test progress: 80 percent"}
{"level":"info","message":"test progress: 90 percent"}
{"level":"info","message":"test progress: 100 percent"}
`
	require.Equal(t, expectedLogs, buf.String())
}

func TestLogProgress1000(t *testing.T) {
	for total := 11; total < 1000; total++ {
		buf := bytes.NewBufferString("")
		lg := zerolog.New(buf)
		logger := LogProgress("test", total, lg)
		for i := 0; i < total; i++ {
			logger(i)
		}

		expectedLogs := `{"level":"info","message":"test progress: 0 percent"}
{"level":"info","message":"test progress: 10 percent"}
{"level":"info","message":"test progress: 20 percent"}
{"level":"info","message":"test progress: 30 percent"}
{"level":"info","message":"test progress: 40 percent"}
{"level":"info","message":"test progress: 50 percent"}
{"level":"info","message":"test progress: 60 percent"}
{"level":"info","message":"test progress: 70 percent"}
{"level":"info","message":"test progress: 80 percent"}
{"level":"info","message":"test progress: 90 percent"}
{"level":"info","message":"test progress: 100 percent"}
`
		require.Equal(t, expectedLogs, buf.String(), total)
	}
}
