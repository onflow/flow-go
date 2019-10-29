# gsip: gRPC Server Interface Parser
gsip is a Flow native tool, which parses a gRPC Server interface into a node
registry compliant code. The node registry represents a map of functions that
can be invoked upon the reception of a Gossip Message. In other words, once a
function is registered into node registry, the node is capable of executing that
upon receiving any Gossip Message that indicates to that function (in its
`method` field). This allows execution of functions to be _gossiped_ over the
network.

Gnode registries adhere to the following interface:
```go
type Registry interface {
  Method() map[string]HandleFunc
}
```

The above interface simply means that to be a node Registry, one should provide
a method which contains all the functions that they wish to serve on a node in
the gossip network.

## Goal of the gossip registry generator script
By using the provided script, one can easily convert an already existing code
base from gRPC services to node registries. Those registries contain a
collection of functions which a node can provide.

Generation of the registry is done as follows:

1.  Reads in the script a file containing the generated gRPC server interface
    (usually ends with ".pb.go").
2.  Extracts the server interface details
3.  Generates a structure which will wrap an instance of the extracted gRPC
    server interface and provide a structure that satisfies node's registry
interface (shown above).

The generated registries can later be fed to node instances via the generated Registry
constructor.

```go
gossip.NewNodeWithRegistry(NewVerifyServiceServerRegistry(&myVerifyServiceServerImp{}))
```

In the example above `myVerifyServiceServerImp` is a structure that we made that
implements the `VerifyServiceServer` interface. We fed it to the generated
registry constructor, and passed the resultant registry to the node constructor.

## Generated code format

To explain what exactly is generated, we will follow a simple example of
transitioning a gRPC server interface into a node registry.

```go
package home

type TVRemoteServer interface {
  TurnOn(context.Context, *Void) (*Void, error)
  TurnOff(context.Context, *Void) (*Void, error)
}
```

After reading the above server interface, the script will extract the two functions that
define the interface.

In order to make them compatible with  the node registry
function signature (of type HandleFunc `func(context.Context, []byte) ([]byte,
error)`), the script generates wrappers for those functions.

```go
TVRemoteServer.TurnOn(context.Context, *Void) (*Void, error)
```

Becomes wrapped in another function,

```go
func (... OurRegistryType) TurnOn(ctx context.Context, payloadByte []byte) ([]byte, error) {
	// Unmarshaling payloadByte to payload
  ...

	resp, respErr := TVRemoteServer.TurnOn(ctx, payload)


	// Marshaling response into respByte
  ...

	return respByte, respErr
}
```

By wrapping the original function signature, into another one which is of type
`HandleFunc`, now we can use it to populate a node registry.


Finally to make our generated structure satisfy the `Registry` interface, we add
the `Methods()` method, which returns the list of functions provided by the
registry.

```
func (tvrsr *TVRemoteServerRegistry) Methods() map[string]gossip.HandleFunc {
	return map[string]gossip.HandleFunc{
		"TurnOn":  tvrsr.TurnOn,
     ...
	}
}
```

Overall, one can see that it is simply a wrapper around the original gRPC server
interface that makes it compatible with gossip networks.

## Usage
1. Build the tool
Go to the flow repo and run the following
```
go build ./network/gossip/scripts/ -o /tmp/generator
```

2. Pass it an argument containing the name of the auto generated gRPC file that
   you would like to convert to a node registry.
```
/tmp/generator ./pkg/gRPC/services/verify/verify.pb.go
```

This will send the auto generated code to the console. In order to save it into
the file you can use the `-w` flag as follows:
```
/tmp/generator -w ./pkg/gRPC/services/verify/verify.pb.go
```

Now a file named `verify_registry.gen.go` which contains the generated registry
will be found in `./pkg/gRPC/services/verify/`, the same folder containing the
passed input file.


## Keywords

* node: Gossip Node
