package main

import (
	"os"
	"runtime"
	"strings"

	"github.com/onflow/flow-go/utils/io"
)

type Environment struct {
	// Short description of the environment.
	EnvironmentName string
	// GoVersion is the version of the Go runtime.
	EnvironmentGoVersion string
	// KernelVersion is the version of the kernel.
	EnvironmentKernelVersion string
}

func defaultEnvironment() Environment {
	kernelFull, _ := io.ReadFile("/proc/version")
	kernelFields := strings.Fields(string(kernelFull))

	var kernelVersion = ""
	if len(kernelFields) >= 3 {
		kernelVersion = kernelFields[2]
	}

	hostname, err := os.Hostname()
	if err != nil {
		hostname = ""
	}

	return Environment{
		EnvironmentName:          hostname,
		EnvironmentGoVersion:     runtime.Version(),
		EnvironmentKernelVersion: kernelVersion,
	}
}
