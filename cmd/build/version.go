// Package build contains information about the build that injected at build-time.
//
// To use this package, simply import it in your program, then add build
// arguments like the following:
//
//	go build -ldflags "-X github.com/onflow/flow-go/cmd/build.semver=v1.0.0"
package build

import (
	"fmt"
	"strings"

	smv "github.com/coreos/go-semver/semver"
)

// Default value for build-time-injected version strings.
const undefined = "undefined"

// The following variables are injected at build-time using ldflags.
var (
	semver string
	commit string
)

// Version returns the raw version string of this build.
func Version() string {
	return semver
}

// Commit returns the commit at which this build was created.
func Commit() string {
	return commit
}

// IsDefined determines whether a version string is defined. Inputs should
// have been produced from this package.
func IsDefined(v string) bool {
	return v != undefined
}

// If any of the build-time-injected variables are empty at initialization,
// mark them as undefined.
func init() {
	if len(semver) == 0 {
		semver = undefined
	}
	if len(commit) == 0 {
		commit = undefined
	}
}

var UndefinedVersionError = fmt.Errorf("version is undefined")

// Semver returns the semantic version of this build as a semver.Version
// if it is defined, or UndefinedVersionError otherwise.
// The version string is converted to a semver compliant one if it isn't already
// but this might fail if the version string is still not semver compliant. In that
// case, an error is returned.
func Semver() (*smv.Version, error) {
	if !IsDefined(semver) {
		return nil, UndefinedVersionError
	}
	ver, err := smv.NewVersion(makeSemverCompliant(semver))
	return ver, err
}

// makeSemverCompliant converts a non-semver version string to a semver compliant one.
// This removes the leading 'v'.
// In the past we sometimes omitted the patch version, e.g. v1.0.0 became v1.0 so this
// also adds a 0 patch version if there's no patch version.
func makeSemverCompliant(version string) string {
	if !IsDefined(version) {
		return version
	}

	// Remove the leading 'v'
	version = strings.TrimPrefix(version, "v")

	// If there's no patch version, add .0
	parts := strings.SplitN(version, "-", 2)
	if strings.Count(parts[0], ".") == 1 {
		parts[0] = parts[0] + ".0"
	}

	version = strings.Join(parts, "-")
	return version
}
