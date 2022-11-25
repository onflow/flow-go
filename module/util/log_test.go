package util

import (
	"testing"

	"github.com/rs/zerolog/log"
)

func TestLogProgress(t *testing.T) {
	logger := LogProgress("test", 41, &log.Logger)
	for i := 0; i < 41; i++ {
		logger(i)
	}
}
