package rest

import (
	"context"
	"net/http"
	"strings"

	"github.com/gorilla/mux"
)

type ctxKeyType string

const expandQueryParam = "expand"
const selectQueryParam = "select"

// commonQueryParamMiddleware generates a Middleware function that extracts the given query parameter from the request
// and adds it to the request context as a key value pair with the key as the query param name.
func commonQueryParamMiddleware(queryParamName string) mux.MiddlewareFunc {
	return func(handler http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			if values, ok := req.URL.Query()[queryParamName]; ok {
				values := strings.Split(values[0], ",")
				// save the query param value in the request context
				contextKey := ctxKeyType(queryParamName)
				req = req.WithContext(context.WithValue(req.Context(), contextKey, values))
			}
			handler.ServeHTTP(w, req)
		})
	}
}

func getField(ctx context.Context, key string) ([]string, bool) {
	contextKey := ctxKeyType(key)
	u, ok := ctx.Value(contextKey).([]string)
	return u, ok
}

// Expandable Middleware extracts out the 'expand' query param field if present in the request
func ExpandableMiddleware() mux.MiddlewareFunc {
	return commonQueryParamMiddleware(expandQueryParam)
}

func GetFieldsToExpand(ctx context.Context) ([]string, bool) {
	return getField(ctx, expandQueryParam)
}

// Select Middleware extracts out the 'select' query param field if present in the request
func SelectMiddleware() mux.MiddlewareFunc {
	return commonQueryParamMiddleware(selectQueryParam)
}

func GetFieldsToSelect(ctx context.Context) ([]string, bool) {
	return getField(ctx, selectQueryParam)
}
