package logging

import (
	"fmt"
	"regexp"
	"strings"
)

var (
	// componentIDRegex matches a valid normalized component ID: dot-separated segments
	// where each segment starts with an alphanumeric character and contains only
	// alphanumeric characters, hyphens, and underscores.
	componentIDRegex = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]*(\.[a-z0-9][a-z0-9_-]*)*$`)

	// wildcardPatternRegex matches a valid normalized wildcard pattern: a component ID
	// prefix followed by ".*".
	wildcardPatternRegex = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]*(\.[a-z0-9][a-z0-9_-]*)*\.\*$`)
)

// NormalizePattern converts s to lowercase, making component ID and pattern matching
// case-insensitive. Must be applied before [ValidateComponentID] or [ValidatePattern].
func NormalizePattern(s string) string {
	return strings.ToLower(s)
}

// ValidateComponentID returns an error if s is not a valid component ID.
// s must already be normalized via [NormalizePattern] before calling this function.
//
// A valid component ID consists of dot-separated segments where each segment starts
// with an alphanumeric character and contains only alphanumeric characters, hyphens,
// and underscores. Examples: "hotstuff", "hotstuff.voter", "consensus.vote-aggregator".
//
// Expected error returns during normal operation:
//   - error: if s does not match the required format.
func ValidateComponentID(s string) error {
	if !componentIDRegex.MatchString(s) {
		return fmt.Errorf("invalid component ID %q: must be dot-separated segments of [a-z0-9_-], each starting with [a-z0-9]", s)
	}
	return nil
}

// ValidatePattern returns an error if s is not a valid component pattern.
// s must already be normalized via [NormalizePattern] before calling this function.
//
// A valid pattern is either a component ID (e.g. "hotstuff.voter") or a prefix wildcard
// (e.g. "hotstuff.*"). The special reset-all token "*" is not validated by this function
// and must be handled separately by callers.
//
// Expected error returns during normal operation:
//   - error: if s does not match the required format.
func ValidatePattern(s string) error {
	if componentIDRegex.MatchString(s) || wildcardPatternRegex.MatchString(s) {
		return nil
	}
	return fmt.Errorf("invalid pattern %q: must be a component ID (e.g. \"hotstuff.voter\") or wildcard (e.g. \"hotstuff.*\")", s)
}
