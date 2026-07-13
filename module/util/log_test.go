package util

import (
	"bytes"
	"fmt"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
)

func TestLogProgress40(t *testing.T) {
	t.Parallel()

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
	for range total {
		logger(1)
	}

	expectedLogs := []string{
		`test progress 0/40 (0.0%)`,
		`test progress 4/40 (10.0%)`,
		`test progress 8/40 (20.0%)`,
		`test progress 12/40 (30.0%)`,
		`test progress 16/40 (40.0%)`,
		`test progress 20/40 (50.0%)`,
		`test progress 24/40 (60.0%)`,
		`test progress 28/40 (70.0%)`,
		`test progress 32/40 (80.0%)`,
		`test progress 36/40 (90.0%)`,
		`test progress 40/40 (100.0%)`,
	}

	for _, log := range expectedLogs {
		require.Contains(t, buf.String(), log, total)
	}
}

func TestLogProgress40By3(t *testing.T) {
	t.Parallel()

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
	for i := 0; i < total/3; i++ {
		logger(3)
	}
	logger(1)

	expectedLogs := []string{
		`test progress 0/40 (0.0%)`,
		`test progress 4/40 (10.0%)`,
		`test progress 8/40 (20.0%)`,
		`test progress 12/40 (30.0%)`,
		`test progress 16/40 (40.0%)`,
		`test progress 20/40 (50.0%)`,
		`test progress 24/40 (60.0%)`,
		`test progress 28/40 (70.0%)`,
		`test progress 32/40 (80.0%)`,
		`test progress 36/40 (90.0%)`,
		`test progress 40/40 (100.0%)`,
	}

	for _, log := range expectedLogs {
		require.Contains(t, buf.String(), log, total)
	}
}

func TestLogProgress43B(t *testing.T) {
	t.Parallel()

	buf := bytes.NewBufferString("")
	lg := zerolog.New(buf)
	total := 43
	logger := LogProgress(
		lg,
		DefaultLogProgressConfig(
			"test",
			total,
		),
	)
	for range total {
		logger(1)
	}

	expectedLogs := []string{
		`test progress 0/43 (0.0%)`,
		`test progress 7/43`,
		`test progress 11/43`,
		`test progress 15/43`,
		`test progress 19/43`,
		`test progress 23/43`,
		`test progress 27/43`,
		`test progress 31/43`,
		`test progress 35/43`,
		`test progress 39/43`,
		`test progress 43/43 (100.0%)`,
	}

	for _, log := range expectedLogs {
		require.Contains(t, buf.String(), log, total)
	}
}

func TestLogProgress43By3(t *testing.T) {
	t.Parallel()

	buf := bytes.NewBufferString("")
	lg := zerolog.New(buf)
	total := 43
	logger := LogProgress(
		lg,
		DefaultLogProgressConfig(
			"test",
			total,
		),
	)
	for i := 0; i < total/3; i++ {
		logger(3)
	}
	logger(1)

	expectedLogs := []string{
		`test progress 0/43 (0.0%)`,
		`test progress 7/43`,
		`test progress 11/43`,
		`test progress 15/43`,
		`test progress 19/43`,
		`test progress 23/43`,
		`test progress 27/43`,
		`test progress 31/43`,
		`test progress 35/43`,
		`test progress 39/43`,
		`test progress 43/43 (100.0%)`,
	}

	for _, log := range expectedLogs {
		require.Contains(t, buf.String(), log, total)
	}
}

func TestLog100WhenOvershooting(t *testing.T) {
	t.Parallel()

	buf := bytes.NewBufferString("")
	lg := zerolog.New(buf)
	total := 100
	logger := LogProgress(
		lg,
		DefaultLogProgressConfig(
			"test",
			total,
		),
	)
	for i := 0; i < total/3+1; i++ {
		logger(3)
	}

	expectedLogs := []string{
		`test progress 0/100 (0.0%)`,
		`test progress 100/100 (100.0%)`,
	}

	for _, log := range expectedLogs {
		require.Contains(t, buf.String(), log, total)
	}
	lines := strings.Count(buf.String(), "\n")
	// every 10% + 1 for the final log
	require.Equal(t, 11, lines)
}

func TestLogProgress1000(t *testing.T) {
	t.Parallel()

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
			fmt.Sprintf(`test progress 0/%d`, total),
			fmt.Sprintf(`test progress %d/%d (100.0%%)`, total, total),
		}

		for _, log := range expectedLogs {
			require.Contains(t, buf.String(), log, total)
		}
	}
}

func TestLogProgressWhenTotalIs0(t *testing.T) {
	t.Parallel()

	buf := bytes.NewBufferString("")
	lg := zerolog.New(buf)
	logger := LogProgress(
		lg,
		DefaultLogProgressConfig(
			"test",
			0,
		),
	)

	for range 10 {
		logger(1)
	}

	expectedLogs := []string{
		fmt.Sprintf(`test progress %d/%d (100.0%%)`, 0, 0),
	}

	for _, log := range expectedLogs {
		require.Contains(t, buf.String(), log, 0)
	}

	lines := strings.Count(buf.String(), "\n")
	// log only once
	require.Equal(t, 1, lines)
}

func TestLogProgressMoreTicksThenTotal(t *testing.T) {
	t.Parallel()

	buf := bytes.NewBufferString("")
	lg := zerolog.New(buf)
	logger := LogProgress(
		lg,
		DefaultLogProgressConfig(
			"test",
			5,
		),
	)

	for range 5 {
		logger(1)
	}

	expectedLogs := []string{
		fmt.Sprintf(`test progress %d/%d (100.0%%)`, 5, 5),
	}

	for _, log := range expectedLogs {
		require.Contains(t, buf.String(), log, 0)
	}

	lines := strings.Count(buf.String(), "\n")
	// log only once
	require.Equal(t, 6, lines)
}

func TestLogProgressContinueLoggingAfter100(t *testing.T) {
	t.Parallel()

	buf := bytes.NewBufferString("")
	lg := zerolog.New(buf)
	logger := LogProgress(
		lg,
		DefaultLogProgressConfig(
			"test",
			100,
		),
	)

	for range 15 {
		logger(10)
	}

	expectedLogs := []string{
		fmt.Sprintf(`test progress %d/%d (100.0%%)`, 100, 100),
		fmt.Sprintf(`test progress %d/%d (110.0%%)`, 110, 100),
		fmt.Sprintf(`test progress %d/%d (150.0%%)`, 150, 100),
	}

	for _, log := range expectedLogs {
		require.Contains(t, buf.String(), log, 0)
	}

	lines := strings.Count(buf.String(), "\n")
	// log only once
	require.Equal(t, 16, lines)
}

func TestLogProgressNoDataForAWhile(t *testing.T) {
	t.Parallel()

	total := 1000

	buf := bytes.NewBufferString("")
	lg := zerolog.New(buf)
	logger := LogProgress(
		lg,
		NewLogProgressConfig[uint64](
			"test",
			uint64(total),
			1*time.Millisecond,
			10,
		),
	)

	for i := range total {
		// somewhere in the middle pause for a bit
		if i == 13 {
			<-time.After(3 * time.Millisecond)
		}

		logger(1)
	}

	expectedLogs := []string{
		fmt.Sprintf(`test progress 0/%d`, total),
		fmt.Sprintf(`test progress %d/%d (100.0%%)`, total, total),
	}

	for _, log := range expectedLogs {
		require.Contains(t, buf.String(), log, total)
	}
	lines := strings.Count(buf.String(), "\n")
	// every 10% + 1 for the final log + 1 for the "no data in a while" log
	require.Equal(t, 12, lines)
}

func TestLogProgressMultipleGoroutines(t *testing.T) {
	t.Parallel()

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
	for range 10 {
		wg.Go(func() {
			for range 100 {
				logger(1)
			}
		})
	}

	wg.Wait()

	expectedLogs := []string{
		fmt.Sprintf(`test progress 0/%d`, total),
		fmt.Sprintf(`test progress %d/%d (100.0%%)`, total, total),
	}

	lines := strings.Count(buf.String(), "\n")
	// every 10% + 1 for the final log
	require.Equal(t, 11, lines)

	for _, log := range expectedLogs {
		require.Contains(t, buf.String(), log, total)
	}
}

func BenchmarkLogProgress(b *testing.B) {
	l := LogProgress(zerolog.New(io.Discard), DefaultLogProgressConfig("test", b.N))
	for i := 0; i < b.N; i++ {
		l(1)
	}
}
