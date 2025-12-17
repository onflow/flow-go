package router

import (
	"fmt"
	"net/http"
	"regexp"
	"strings"
)

// For each compiled route regex store a map[method]routeName
var routePatterns []*regexp.Regexp
var routeMethodNameMap map[*regexp.Regexp]map[string]string

func init() {
	routePatterns = make([]*regexp.Regexp, 0, len(Routes)+len(WSLegacyRoutes))
	routeMethodNameMap = make(map[*regexp.Regexp]map[string]string)

	// Deduplicate compiled regex by pattern string (for GET/POST same path)
	regexByPattern := make(map[string]*regexp.Regexp)

	add := func(method, pattern, name string) {
		// Compile one regex per unique path pattern
		regexPattern := "^" + patternToRegex(pattern) + "$"

		re, ok := regexByPattern[regexPattern]
		if !ok {
			re = regexp.MustCompile(regexPattern)
			regexByPattern[regexPattern] = re
			routePatterns = append(routePatterns, re)
			routeMethodNameMap[re] = make(map[string]string)
		}

		routeMethodNameMap[re][method] = name
	}

	for _, r := range Routes {
		add(r.Method, r.Pattern, r.Name)
	}
	for _, r := range WSLegacyRoutes {
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

func MethodURLToRoute(method, url string) (string, error) {
	path := strings.TrimPrefix(url, "/v1")

	for _, pattern := range routePatterns {
		if pattern.MatchString(path) {
			if byMethod, ok := routeMethodNameMap[pattern]; ok {
				if name, ok := byMethod[method]; ok {
					return name, nil
				}
				return "", fmt.Errorf("no matching route found for method %s and URL: %s", method, url)
			}
		}
	}

	return "", fmt.Errorf("no matching route found for URL: %s", url)
}

// URLToRoute matches the URL against route patterns and returns the matching route name
func URLToRoute(id string) (string, error) {
	method := http.MethodGet
	path := id

	if sp := strings.IndexByte(id, ' '); sp > 0 {
		maybeMethod := id[:sp]
		maybePath := strings.TrimSpace(id[sp+1:])
		if strings.HasPrefix(maybePath, "/") {
			method = maybeMethod
			path = maybePath
		}
	}

	return MethodURLToRoute(method, path)
}
