package router

import (
	"fmt"
	"regexp"
	"strings"
)

// the following logic is used to match the URL with the correct route for metrics collection.

var routePatterns []*regexp.Regexp
var routeNameMap map[*regexp.Regexp]string

func init() {
	routePatterns = make([]*regexp.Regexp, 0, len(Routes)+len(WSLegacyRoutes))
	routeNameMap = make(map[*regexp.Regexp]string)

	// Convert REST route patterns to regex patterns for matching
	for _, r := range Routes {
		regexPattern := patternToRegex(r.Pattern)
		re := regexp.MustCompile("^" + regexPattern + "$")
		routePatterns = append(routePatterns, re)
		routeNameMap[re] = r.Name
	}

	// Convert WebSocket route patterns to regex patterns for matching
	for _, r := range WSLegacyRoutes {
		regexPattern := patternToRegex(r.Pattern)
		re := regexp.MustCompile("^" + regexPattern + "$")
		routePatterns = append(routePatterns, re)
		routeNameMap[re] = r.Name
	}
}

// patternToRegex converts a mux pattern like "/blocks/{id}" to a regex pattern
func patternToRegex(pattern string) string {
	// Escape special regex characters except for {}
	escaped := regexp.QuoteMeta(pattern)
	// Replace placeholder patterns with regex matchers
	// {id} -> matches 64 char hex string or integer
	escaped = strings.ReplaceAll(escaped, `\{id\}`, `([0-9a-fA-F]{64}|\d+)`)
	// {address} -> matches 16 char hex string
	escaped = strings.ReplaceAll(escaped, `\{address\}`, `[0-9a-fA-F]{16}`)
	// {index} -> matches integer
	escaped = strings.ReplaceAll(escaped, `\{index\}`, `\d+`)
	return escaped
}

// URLToRoute matches the URL against route patterns and returns the matching route name
func URLToRoute(url string) (string, error) {
	path := strings.TrimPrefix(url, "/v1")
	for _, pattern := range routePatterns {
		if pattern.MatchString(path) {
			return routeNameMap[pattern], nil
		}
	}
	return "", fmt.Errorf("no matching route found for URL: %s", url)
}
