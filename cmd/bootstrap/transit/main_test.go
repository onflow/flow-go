package main

func Example() {

	undefined := "undefined"
	validVersion := "v0.0.1"
	validCommit := "fa3f1af8f007940717758e63709104101380218f"

	var tests = []struct {
		semver, commit string
	}{
		{undefined, undefined},
		{undefined, validCommit},
		{validVersion, undefined},
		{validVersion, validCommit},
	}
	for _, t := range tests {
		semver = t.semver
		commit = t.commit
		printVersion()
	}

	// Output:
	// Transit script version information unknown
	// Transit script Commit: fa3f1af8f007940717758e63709104101380218f
	// Transit script Version: v0.0.1
	// Transit script Version: v0.0.1
	// Transit script Commit: fa3f1af8f007940717758e63709104101380218f
}
