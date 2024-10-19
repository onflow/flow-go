package emulator

import (
	"github.com/logrusorgru/aurora"
	"github.com/rs/zerolog"
	"strings"
)

type CadenceHook struct {
	MainLogger *zerolog.Logger
}

func (h CadenceHook) Run(_ *zerolog.Event, level zerolog.Level, msg string) {
	const logPrefix = "Cadence log:"
	if level != zerolog.NoLevel && strings.HasPrefix(msg, logPrefix) {
		h.MainLogger.Info().Msg(
			strings.Replace(msg,
				logPrefix,
				aurora.Colorize("LOG:", aurora.BlueFg|aurora.BoldFm).String(),
				1))
	}
}
