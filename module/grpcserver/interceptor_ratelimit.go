package grpcserver

import (
	"context"
	"path/filepath"

	"github.com/rs/zerolog"
	"golang.org/x/time/rate"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const defaultRateLimit = 1000 // aggregate default rate limit for all unspecified API calls
const defaultBurst = 100      // default burst limit (calls made at the same time) for an API

// RateLimiterInterceptor is a gRPC interceptor that applies rate limits to incoming requests.
type RateLimiterInterceptor struct {
	log zerolog.Logger

	// a shared default rate limiter for APIs whose rate limit is not explicitly defined
	defaultLimiter *rate.Limiter

	// a map of api and its limiter
	methodLimiterMap map[string]*rate.Limiter
}

// NewRateLimiterInterceptor creates a new rate limiter interceptor with the defined per second rate
// limits and the optional burst limit for each API.
func NewRateLimiterInterceptor(log zerolog.Logger, apiRateLimits map[string]int, apiBurstLimits map[string]int) *RateLimiterInterceptor {
	defaultLimiter := rate.NewLimiter(rate.Limit(defaultRateLimit), defaultBurst)
	methodLimiterMap := make(map[string]*rate.Limiter, len(apiRateLimits))

	// read rate limit values for each API and create a limiter for each
	for api, limit := range apiRateLimits {
		// if a burst limit is defined for this api, use that else use the default
		burst := defaultBurst
		if b, ok := apiBurstLimits[api]; ok {
			burst = b
		}
		methodLimiterMap[api] = rate.NewLimiter(rate.Limit(limit), burst)
	}

	if len(methodLimiterMap) == 0 {
		log.Info().Int("default_rate_limit", defaultRateLimit).Msg("no rate limits specified, using the default limit")
	}

	return &RateLimiterInterceptor{
		defaultLimiter:   defaultLimiter,
		methodLimiterMap: methodLimiterMap,
		log:              log,
	}
}

// UnaryServerInterceptor returns a grpc.UnaryServerInterceptor that applies rate limits to request
// based on the limits defined when creating the RateLimiterInterceptor
func (interceptor *RateLimiterInterceptor) UnaryServerInterceptor(
	ctx context.Context,
	req any,
	info *grpc.UnaryServerInfo,
	handler grpc.UnaryHandler,
) (resp any, err error) {
	// remove the package name (e.g. "/flow.access.AccessAPI/Ping" to "Ping")
	methodName := filepath.Base(info.FullMethod)

	limiter, ok := interceptor.methodLimiterMap[methodName]
	if !ok {
		interceptor.log.Trace().Str("method", methodName).Msg("rate limit not defined, using default limit")
		limiter = interceptor.defaultLimiter
	}

	if !limiter.Allow() {
		interceptor.log.Trace().
			Str("method", methodName).
			Interface("request", req).
			Float64("limit", float64(limiter.Limit())).
			Msg("rate limit exceeded")

		return nil, status.Errorf(codes.ResourceExhausted, "%s rate limit reached, please retry later.",
			info.FullMethod)
	}

	return handler(ctx, req)
}
