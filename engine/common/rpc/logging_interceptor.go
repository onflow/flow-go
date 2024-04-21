package rpc

import (
	"context"
	"fmt"

	"github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/logging"
	"github.com/rs/zerolog"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
)

func customClientCodeToLevel(c codes.Code) logging.Level {
	if c == codes.OK {
		// log successful returns as Debug to avoid excessive logging in info mode
		return logging.DEBUG
	}
	return logging.DefaultServerCodeToLevel(c)
}

// LoggingInterceptor creates the logging interceptors to log incoming GRPC request and response (minus the payload body)
func LoggingInterceptor(log zerolog.Logger) []grpc.UnaryServerInterceptor {
	loggingInterceptor := logging.UnaryServerInterceptor(InterceptorLogger(log), logging.WithLevels(customClientCodeToLevel))
	return []grpc.UnaryServerInterceptor{loggingInterceptor}
}

// InterceptorLogger adapts zerolog logger to interceptor logger.
// This code is simple enough to be copied and not imported.
func InterceptorLogger(l zerolog.Logger) logging.Logger {
	return logging.LoggerFunc(func(ctx context.Context, lvl logging.Level, msg string, fields ...any) {
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
