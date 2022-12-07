package util

import (
	"testing"

	"github.com/rs/zerolog/log"
)

func TestLogProgress(t *testing.T) {
	logger := LogProgress("test", 40, &log.Logger)
	for i := 0; i < 50; i++ {
		logger(i)
	}
}
