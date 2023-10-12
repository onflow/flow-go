package util

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
)

func TestLogProgress40(t *testing.T) {
	buf := bytes.NewBufferString("")
	lg := zerolog.New(buf)
	total := 40
	logger := LogProgress(lg, "test", total)
	for i := 0; i < total; i++ {
		logger(1)
	}

	expectedLogs := []string{
		`test progress 1/40 (2.5%)`,
		`test progress 5/40 (12.5%)`,
		`test progress 9/40 (22.5%)`,
		`test progress 13/40 (32.5%)`,
		`test progress 17/40 (42.5%)`,
		`test progress 21/40 (52.5%)`,
		`test progress 25/40 (62.5%)`,
		`test progress 29/40 (72.5%)`,
		`test progress 33/40 (82.5%)`,
		`test progress 37/40 (92.5%)`,
		`test progress 40/40 (100.0%)`,
	}

	for _, log := range expectedLogs {
		require.Contains(t, buf.String(), log, total)
	}
}

func TestLogProgress1000(t *testing.T) {
	for total := 11; total < 1000; total++ {
		buf := bytes.NewBufferString("")
		lg := zerolog.New(buf)
		logger := LogProgress(lg, "test", total)
		for i := 0; i < total; i++ {
			logger(1)
		}

		expectedLogs := []string{
			fmt.Sprintf(`test progress 1/%d`, total),
			fmt.Sprintf(`test progress %d/%d (100.0%%)`, total, total),
		}

		for _, log := range expectedLogs {
			require.Contains(t, buf.String(), log, total)
		}
	}
}
