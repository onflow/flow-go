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

func MustGetDefaultEnvironment() Environment {
	kernelFull, err := io.ReadFile("/proc/version")
	if err != nil {
		panic(err)
	}

	var kernelVersion = "unknown"
	kernelFields := strings.Fields(string(kernelFull))
	if len(kernelFields) >= 3 {
		kernelVersion = kernelFields[2]
	}

	hostname, err := os.Hostname()
	if err != nil {
		panic(err)
	}

	return Environment{
		EnvironmentName:          hostname,
		EnvironmentGoVersion:     runtime.Version(),
		EnvironmentKernelVersion: kernelVersion,
	}
}
