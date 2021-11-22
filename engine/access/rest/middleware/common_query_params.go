package middleware

import (
	"net/http"
	"strings"

	"github.com/gorilla/mux"
)

const expandQueryParam = "expand"
const selectQueryParam = "select"

// commonQueryParamMiddleware generates a Middleware function that extracts the given query parameter from the request
// and adds it to the request context as a key value pair. The key is the query param name and the value as a string slice containing
// all comma separated values.
// e.g. for queryParamName "field", if the request url contains <some url>?field=value1,value2,...valueN
// the middleware returned by commonQueryParamMiddleware will add the key - "field" to the request context with value
// []string{"value1", "value2",..."valueN"} when it is executed
func commonQueryParamMiddleware(queryParamName string) mux.MiddlewareFunc {
	return func(handler http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			if values, ok := req.URL.Query()[queryParamName]; ok {
				values := strings.Split(values[0], ",")
				// save the query param value in the request context
				req = addRequestAttribute(req, queryParamName, values)
			}
			handler.ServeHTTP(w, req)
		})
	}
}

// QueryExpandable middleware extracts out the 'expand' query param field if present in the request
func QueryExpandable() mux.MiddlewareFunc {
	return commonQueryParamMiddleware(expandQueryParam)
}

// QuerySelect middleware extracts out the 'select' query param field if present in the request
func QuerySelect() mux.MiddlewareFunc {
	return commonQueryParamMiddleware(selectQueryParam)
}

func getField(req *http.Request, key string) ([]string, bool) {
	value, found := getRequestAttribute(req, key)
	if !found {
		return nil, false
	}
	valueAsStringSlice, ok := value.([]string)
	return valueAsStringSlice, ok
}

func GetFieldsToExpand(req *http.Request) ([]string, bool) {
	return getField(req, expandQueryParam)
}

func GetFieldsToSelect(req *http.Request) ([]string, bool) {
	return getField(req, selectQueryParam)
}
