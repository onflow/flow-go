package grpcserver

import (
	"context"
	"fmt"

	"github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/logging"
	"github.com/rs/zerolog"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
)

// LoggingInterceptor returns a grpc.UnaryServerInterceptor that logs incoming GRPC request and response
func LoggingInterceptor(log zerolog.Logger) grpc.UnaryServerInterceptor {
	return logging.UnaryServerInterceptor(
		InterceptorLogger(log),
		logging.WithLevels(statusCodeToLogLevel),
	)
}

// InterceptorLogger adapts a zerolog.Logger to interceptor's logging.Logger
// This code is simple enough to be copied and not imported.
func InterceptorLogger(l zerolog.Logger) logging.Logger {
	return logging.LoggerFunc(func(_ context.Context, lvl logging.Level, msg string, fields ...any) {
		l := l.With().Fields(fields).Logger()

		switch lvl {
		case logging.LevelDebug:
			l.Debug().Msg(msg)
		case logging.LevelInfo:
			l.Info().Msg(msg)
		case logging.LevelWarn:
			l.Warn().Msg(msg)
		case logging.LevelError:
			l.Error().Msg(msg)
		default:
			panic(fmt.Sprintf("unknown level %v", lvl))
		}
	})
}

// statusCodeToLogLevel converts a grpc status.Code to the appropriate logging.Level
func statusCodeToLogLevel(c codes.Code) logging.Level {
	switch c {
	case codes.OK:
		// log successful returns as Debug to avoid excessive logging in info mode
		return logging.LevelDebug
	case codes.DeadlineExceeded, codes.ResourceExhausted, codes.OutOfRange:
		// these are common, map to info
		return logging.LevelInfo
	default:
		return logging.DefaultServerCodeToLevel(c)
	}
}
