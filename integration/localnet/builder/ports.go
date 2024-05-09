package main

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/onflow/flow-go/integration/testnet"
)

// portConfig configures port ranges for all nodes within a particular role.
type portConfig struct {
	// start is the first port to use for this role
	start int
	// end is the first port to use for the next role
	// e.g. the role's range is [start, end)
	end int
	// portCount is the number of ports to allocate for each node
	portCount int
	// nodeCount is the current number of nodes that have been allocated
	nodeCount int
}

var config = map[string]*portConfig{
	"access": {
		start:     4000, // 4000-5000 => 100 nodes
		end:       5000,
		portCount: 10,
	},
	"observer": {
		start:     5001, // 5001-6000 => 100 nodes
		end:       6000,
		portCount: 10,
	},
	"execution": {
		start:     6000, // 6000-6100 => 20 nodes
		end:       6100,
		portCount: 5,
	},
	"collection": {
		start:     6100, // 6100-7100 => 200 nodes
		end:       7100,
		portCount: 5,
	},
	"consensus": {
		start:     7100, // 7100-7600 => 250 nodes
		end:       7600,
		portCount: 2,
	},
	"verification": {
		start:     7600, // 7600-8000 => 200 nodes
		end:       8000,
		portCount: 2,
	},
}

// PortAllocator is responsible for allocating and tracking container-to-host port mappings for each node
type PortAllocator struct {
	exposedPorts   map[string]map[string]string
	availablePorts map[string]int
	nodesNames     []string
}

func NewPortAllocator() *PortAllocator {
	return &PortAllocator{
		exposedPorts:   make(map[string]map[string]string),
		availablePorts: make(map[string]int),
	}
}

// AllocatePorts allocates a block of ports for a given node and role.
func (a *PortAllocator) AllocatePorts(node, role string) error {
	if _, ok := a.availablePorts[node]; ok {
		return fmt.Errorf("container %s already allocated", node)
	}

	c := config[role]

	nodeStart := c.start + c.nodeCount*c.portCount
	if nodeStart >= c.end {
		return fmt.Errorf("no more ports available for role %s", role)
	}

	a.nodesNames = append(a.nodesNames, node)
	a.availablePorts[node] = nodeStart
	c.nodeCount++

	return nil
}

// HostPort returns the host port for a given node and container port.
func (a *PortAllocator) HostPort(node string, containerPort string) string {
	if _, ok := a.exposedPorts[node]; !ok {
		a.exposedPorts[node] = map[string]string{}
	}

	port := fmt.Sprint(a.availablePorts[node])
	a.availablePorts[node]++

	a.exposedPorts[node][containerPort] = port

	return port
}

// WriteMappingConfig writes the port mappings to a JSON file.
func (a *PortAllocator) WriteMappingConfig() error {
	f, err := openAndTruncate(PortMapFile)
	if err != nil {
		return err
	}
	defer f.Close()

	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")

	err = enc.Encode(a.exposedPorts)
	if err != nil {
		return err
	}

	return nil
}

// Print prints the container host port mappings.
func (a *PortAllocator) Print() {
	fmt.Println("Port assignments: [container: host]")
	fmt.Printf("Also available in %s\n", PortMapFile)

	// sort alphabetically, but put observers at the end
	sort.Slice(a.nodesNames, func(i, j int) bool {
		if strings.HasPrefix(a.nodesNames[i], "observer") {
			return false
		}
		return a.nodesNames[i] < a.nodesNames[j]
	})

	for _, node := range a.nodesNames {
		fmt.Printf("  %s:\n", node)
		// print ports in a consistent order
		for _, containerPort := range []string{
			testnet.AdminPort,
			testnet.GRPCPort,
			testnet.GRPCSecurePort,
			testnet.GRPCWebPort,
			testnet.RESTPort,
			testnet.ExecutionStatePort,
			testnet.PublicNetworkPort,
		} {
			if hostPort, ok := a.exposedPorts[node][containerPort]; ok {
				fmt.Printf("    %14s (%s): %s\n", portName(containerPort), containerPort, hostPort)
			}
		}
	}
}

// portName returns a human-readable name for a given container port.
func portName(containerPort string) string {
	switch containerPort {
	case testnet.GRPCPort:
		return "GRPC"
	case testnet.GRPCSecurePort:
		return "Secure GRPC"
	case testnet.GRPCWebPort:
		return "GRPC-Web"
	case testnet.RESTPort:
		return "REST"
	case testnet.ExecutionStatePort:
		return "Execution Data"
	case testnet.AdminPort:
		return "Admin"
	case testnet.PublicNetworkPort:
		return "Public Network"
	default:
		return "Unknown"
	}
}
