package util

import (
	"bytes"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
)

func TestLogProgress40(t *testing.T) {
	buf := bytes.NewBufferString("")
	lg := zerolog.New(buf)
	total := 40
	logger := LogProgress(
		lg,
		DefaultLogProgressConfig(
			"test",
			total,
		),
	)
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
		logger := LogProgress(
			lg,
			DefaultLogProgressConfig(
				"test",
				total,
			),
		)

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

func TestLogProgressMultipleGoroutines(t *testing.T) {
	total := 1000

	buf := bytes.NewBufferString("")
	lg := zerolog.New(buf)
	logger := LogProgress(
		lg,
		DefaultLogProgressConfig(
			"test",
			total,
		),
	)

	wg := sync.WaitGroup{}
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				logger(1)
			}
		}()
	}

	wg.Wait()

	expectedLogs := []string{
		fmt.Sprintf(`test progress 1/%d`, total),
		fmt.Sprintf(`test progress %d/%d (100.0%%)`, total, total),
	}

	lines := strings.Count(buf.String(), "\n")
	// every 10% + 1 for the final log
	require.Equal(t, 11, lines)

	for _, log := range expectedLogs {
		require.Contains(t, buf.String(), log, total)
	}
}

func TestLogProgressCustomSampler(t *testing.T) {
	total := 1000

	nth := uint32(total / 10) // sample every 10% by default
	if nth == 0 {
		nth = 1
	}
	sampler := newProgressLogsSampler(nth, 10*time.Millisecond)

	buf := bytes.NewBufferString("")
	lg := zerolog.New(buf)
	logger := LogProgress(
		lg,
		NewLogProgressConfig(
			"test",
			total,
			sampler,
		),
	)

	for i := 0; i < total; i++ {
		if i == 7 || i == 77 || i == 777 {
			// s
			time.Sleep(20 * time.Millisecond)
		}
		logger(1)
	}

	expectedLogs := []string{
		fmt.Sprintf(`test progress 1/%d`, total),
		fmt.Sprintf(`test progress %d/%d (100.0%%)`, total, total),
	}

	lines := strings.Count(buf.String(), "\n")
	// every 10% + 1 for the final log + 3 for the custom sampler
	require.Equal(t, 14, lines)

	for _, log := range expectedLogs {
		require.Contains(t, buf.String(), log, total)
	}
}
