package rpc

import (
	"context"
	"path/filepath"

	"github.com/rs/zerolog"
	"golang.org/x/time/rate"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var APILimit = map[string]uint{
	"Ping": 10,
}

const defaultRateLimit = 10 // 10 calls per second
const defaultBurst = 3      // at most 3 call that are made at the same time

type rateLimiterInterceptor struct {
	log zerolog.Logger
	ml  *methodLimiter
}

func NewRateLimiterInterceptor(log zerolog.Logger) *rateLimiterInterceptor {

	return &rateLimiterInterceptor{
		ml:  newMethodLimiter(log),
		log: log,
	}
}

func (interceptor *rateLimiterInterceptor) unaryServerInterceptor(ctx context.Context,
	req interface{},
	info *grpc.UnaryServerInfo,
	handler grpc.UnaryHandler) (resp interface{}, err error) {

	if !interceptor.ml.allow(info.FullMethod) {

		// log the limit violation
		interceptor.log.Info().
			Str("method", info.FullMethod).
			Interface("request", req).
			Msg("rate limit exceeded")

		// reject the request
		return nil, status.Errorf(codes.ResourceExhausted, "%s rate limit reached, please retry later.",
			info.FullMethod)
	}

	// call the handler
	h, err := handler(ctx, req)

	return h, err
}

type methodLimiter struct {
	log              zerolog.Logger
	defaultLimiter   *rate.Limiter            // a limiter applied to all unknown method names
	methodLimiterMap map[string]*rate.Limiter // map of method name to its corresponding limiter
}

func newMethodLimiter(log zerolog.Logger) *methodLimiter {

	defaultLimiter := rate.NewLimiter(defaultRateLimit, defaultBurst)

	methodLimiterMap := make(map[string]*rate.Limiter, len(APILimit))
	for api, limit := range APILimit {
		methodLimiterMap[api] = rate.NewLimiter(rate.Limit(limit), defaultBurst)
	}

	ml := &methodLimiter{
		log:              log,
		defaultLimiter:   defaultLimiter,
		methodLimiterMap: methodLimiterMap,
	}
	return ml
}

func (ml *methodLimiter) allow(fullMethodName string) bool {

	// remove the package name
	methodName := filepath.Base(fullMethodName)

	// look up the limiter
	if limiter, ok := ml.methodLimiterMap[methodName]; ok {
		// query the limiter
		return limiter.Allow()
	}

	ml.log.Error().Str("method", methodName).Msg("rate limited not defined, using default limit")
	return ml.defaultLimiter.Allow()
}
