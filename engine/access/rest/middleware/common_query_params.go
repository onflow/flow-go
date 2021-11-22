package middleware

import (
	"net/http"
	"strings"

	"github.com/gorilla/mux"
)

const expandQueryParam = "expand"
const selectQueryParam = "select"

// commonQueryParamMiddleware generates a Middleware function that extracts the given query parameter from the request
// and adds it to the request context as a key value pair with the key as the query param name.
// e.g. for queryParamName "fields", if the request url contains <some url>?fields=field1,field2,..fieldN,
// the middleware returned by commonQueryParamMiddleware will add the key - "fields" to the request context with value
// ["field", "fields2",..."fieldn"] when it is executed
func commonQueryParamMiddleware(queryParamName string) mux.MiddlewareFunc {
	return func(handler http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			if values, ok := req.URL.Query()[queryParamName]; ok {
				values := strings.Split(values[0], ",")
				valueMap := make(map[string]bool, len(values))
				for _, v := range values {
					valueMap[v] = true
				}
				// save the query param value in the request context
				req = addRequestAttribute(req, queryParamName, valueMap)
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

func getField(req *http.Request, key string) (map[string]bool, bool) {
	value, found := getRequestAttribute(req, key)
	if !found {
		return nil, false
	}
	valueAsStringSlice, ok := value.(map[string]bool)
	return valueAsStringSlice, ok
}

func GetFieldsToExpand(req *http.Request) (map[string]bool, bool) {
	return getField(req, expandQueryParam)
}

func GetFieldsToSelect(req *http.Request) (map[string]bool, bool) {
	return getField(req, selectQueryParam)
}
