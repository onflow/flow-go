package router

import (
	"fmt"
	"regexp"
	"strings"
)

// routeMatcher ties together an HTTP method with a compiled regex for the path and the route name.
type routeMatcher struct {
	method string
	re     *regexp.Regexp
	name   string
}

var matchers []routeMatcher

func init() {
	matchers = make([]routeMatcher, 0, len(Routes)+len(WSLegacyRoutes)+len(ExperimentalRoutes))

	add := func(method, pattern, name string) {
		regexPattern := "^" + patternToRegex(pattern) + "$"
		matchers = append(matchers, routeMatcher{
			method: method,
			re:     regexp.MustCompile(regexPattern),
			name:   name,
		})
	}

	for _, r := range Routes {
		add(r.Method, r.Pattern, r.Name)
	}
	for _, r := range WSLegacyRoutes {
		add(r.Method, r.Pattern, r.Name)
	}
	for _, r := range ExperimentalRoutes {
		add(r.Method, r.Pattern, r.Name)
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

// MethodURLToRoute matches (method, url) against compiled route regexes and returns the route name.
func MethodURLToRoute(method, url string) (string, error) {

	path, found := strings.CutPrefix(url, "/v1")
	if !found {
		path = strings.TrimPrefix(url, "/experimental/v1")
	}

	if method == "" {
		for _, m := range matchers {
			if m.re.MatchString(path) {
				return m.name, nil
			}
		}
		return "", fmt.Errorf("no matching route found for URL: %s", url)
	}

	for _, m := range matchers {
		if m.method != method {
			continue
		}
		if m.re.MatchString(path) {
			return m.name, nil
		}
	}

	return "", fmt.Errorf("no matching route found for method %s and URL: %s", method, url)
}
