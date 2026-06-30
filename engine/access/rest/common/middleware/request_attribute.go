package middleware

import (
	"context"
	"net/http"
)

type ctxKeyType string

// addRequestAttribute adds the given attribute name and value to the request context
func addRequestAttribute(req *http.Request, attributeName string, attributeValue any) *http.Request {
	contextKey := ctxKeyType(attributeName)
	return req.WithContext(context.WithValue(req.Context(), contextKey, attributeValue))
}

// getRequestAttribute returns the value for the given attribute name from the request context if found
func getRequestAttribute(req *http.Request, attributeName string) (any, bool) {
	contextKey := ctxKeyType(attributeName)
	value := req.Context().Value(contextKey)
	return value, value != nil
}
