package rpc

import (
	grpczerolog "github.com/grpc-ecosystem/go-grpc-middleware/providers/zerolog/v2"
	"github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/logging"
	"github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/tags"
	"github.com/rs/zerolog"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
)

func customClientCodeToLevel(c codes.Code) logging.Level {
	if c == codes.OK {
		// log successful returns as Debug to avoid excessive logging in info mode
		return logging.DEBUG
	}
	return logging.DefaultClientCodeToLevel(c)
}

// loggingInterceptor creates the logging interceptors
func loggingInterceptor(log zerolog.Logger) []grpc.UnaryServerInterceptor {
	tagsInterceptor := tags.UnaryServerInterceptor(tags.WithFieldExtractor(tags.CodeGenRequestFieldExtractor))
	loggingInterceptor := logging.UnaryServerInterceptor(grpczerolog.InterceptorLogger(log), logging.WithLevels(customClientCodeToLevel))
	return []grpc.UnaryServerInterceptor{tagsInterceptor, loggingInterceptor}
}
