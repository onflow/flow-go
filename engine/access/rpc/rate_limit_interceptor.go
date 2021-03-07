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

var defaultRateLimit = 1000 // aggregate default rate limit for all unspecified API calls
var DefaultBurst = 100      // default burst limit (calls made at the same time) for an API

// rateLimiterInterceptor rate limits the
type rateLimiterInterceptor struct {
	log zerolog.Logger

	// default rate limiter for APIs whose rate limit is not explicitly defined
	defaultLimiter *rate.Limiter

	// a map of api and its limiter
	methodLimiterMap map[string]*rate.Limiter
}

func NewRateLimiterInterceptor(log zerolog.Logger, apiRateLimits map[string]int) *rateLimiterInterceptor {

	defaultLimiter := rate.NewLimiter(rate.Limit(defaultRateLimit), DefaultBurst)
	methodLimiterMap := make(map[string]*rate.Limiter, len(apiRateLimits))

	// read rate limit values for each API and create a limiter for each
	for api, limit := range apiRateLimits {
		methodLimiterMap[api] = rate.NewLimiter(rate.Limit(limit), DefaultBurst)
	}

	if len(methodLimiterMap) == 0 {
		log.Info().Int("default_rate_limit", defaultRateLimit).Msg("no rate limits specified, using the default limit")
	}

	return &rateLimiterInterceptor{
		defaultLimiter:   defaultLimiter,
		methodLimiterMap: methodLimiterMap,
		log:              log,
	}
}

func (interceptor *rateLimiterInterceptor) unaryServerInterceptor(ctx context.Context,
	req interface{},
	info *grpc.UnaryServerInfo,
	handler grpc.UnaryHandler) (resp interface{}, err error) {

	// remove the package name (e.g. "/flow.access.AccessAPI/Ping" to "Ping")
	methodName := filepath.Base(info.FullMethod)

	// look up the limiter
	limiter := interceptor.methodLimiterMap[methodName]

	// if not found, use the default limiter
	if limiter == nil {

		// log error since each API should typically have a defined rate limit
		interceptor.log.Error().Str("method", methodName).Msg("rate limit not defined, using default limit")

		limiter = interceptor.defaultLimiter
	}

	// check if request within limit
	if !limiter.Allow() {

		// reject the request
		return nil, status.Errorf(codes.ResourceExhausted, "%s rate limit reached, please retry later.",
			info.FullMethod)
	}

	// call the handler
	h, err := handler(ctx, req)

	return h, err
}
