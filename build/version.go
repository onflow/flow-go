// Package build contains information about the build that injected at build-time.
//
// To use this package, simply import it in your program, then add build
// arguments like the following:
//
//   go build -ldflags "-X github.com/dapperlabs/flow-go/version.semver=v1.0.0"
package build

// Default value for build-time-injected version strings.
const undefined = "undefined"

// The following variables are injected at build-time using ldflags.
var (
	semver string
	commit string
)

// Semver returns the semantic version of this build.
func Semver() string {
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
		semver = undefined
	}
}
