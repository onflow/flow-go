# gsip: gRPC Server Interface Parser
gsip is a Flow native tool, which parses a gRPC Server interface into a gnode
registry compliant code. The gnode registry represents a map of functions that
can be invoked upon the reception of a Gossip Message. In other words, once a
function is registered into gnode registry, the node is capable of executing that
upon receiving any Gossip Message that indicates to that function (in its
`method` field). This allows execution of functions to be _gossiped_ over the
network.

Gnode registries adhere to the following interface:
```go
type Registry interface {
  Method() map[string]HandleFunc
}
```

The above interface simply means that to be a gnode Registry, one should provide
a method which contains all the functions that they wish to serve on a gnode in
the gossip network.

## Goal of the gossip registry generator script
By using the provided script, one can easily convert an already existing code
base from gRPC services to gnode registries. Those registries contain a
collection of functions which a gnode can provide.

Generation of the registry is done as follows: 

1.  Reads in the script a file containing the generated gRPC server interface
    (usually ends with ".pb.go").
2.  Extracts the server interface details
3.  Generates a structure which will wrap an instance of the extracted gRPC
    server interface and provide a structure that satisfies gnode's registry
interface (shown above). 

The generated registries can later be fed to gnode instances via the generated Registry
constructor. 

```go
gnode.NewNodeWithRegistry(NewVerifyServiceServerRegistry(&myVerifyServiceServerImp{}))
```

In the example above `myVerifyServiceServerImp` is a structure that we made that
implements the `VerifyServiceServer` interface. We fed it to the generated
registry constructor, and passed the resultant registry to the gnode constructor.

## Generated code format

To explain what exactly is generated, we will follow a simple example of
transitioning a gRPC server interface into a gnode registry.

```go
package home

type TVRemoteServer interface {
  TurnOn(context.Context, *Void) (*Void, error)
  TurnOff(context.Context, *Void) (*Void, error)
}
```

After reading the above server interface, the script will extract the two functions that
define the interface. 

In order to make them compatible with  the gnode registry
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
`HandleFunc`, now we can use it to populate a gnode registry.


Finally to make our generated structure satisfy the `Registry` interface, we add
the `Methods()` method, which returns the list of functions provided by the
registry.

```
func (tvrsr *TVRemoteServerRegistry) Methods() map[string]gnode.HandleFunc {
	return map[string]gnode.HandleFunc{
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
go build ./pkg/network/gossip/v1/scripts/ -o /tmp/generator
```

2. Pass it an argument containing the name of the auto generated gRPC file that
   you would like to convert to a gnode registry.
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

* gnode: Gossip Node
