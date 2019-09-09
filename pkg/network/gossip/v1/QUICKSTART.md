# Gossip V1

## Quick Start

The gossip layer enables nodes to multicast a message to a subset of other nodes. The message contains a `payload` and a `method` tag. 
The receiving nodes are supposed to have a consistent implementation of the `method` and to invoke it over the `payload`. 


### Step 1: Importing the Gossip v1 package

Use the following import path in your go file:
```go
import (
	...
	gnode "github.com/dapperlabs/bamboo-node/pkg/network/gossip/v1"
	...
)
```

### Step 2: Declaring a node
A Gossip node is created through the `NewNode` constructor. The node is initially **inactive** in the sense that 
it is not bound to any port and hence does not listen to any connection. 
```go
node := gnode.NewNode()
```

## Step 3: Registering functions
Registering a function corresponds to having the implementation of the function available as a (tag, definition) pair where 
the tag is the name of the function and definition is the function body as an anonymous function.
A function is registered by calling the `RegisterFunc` method on the node.

```go
f := func(msg *shared.GossipMessage) (*shared.MessageReply, error) {
	return &shared.MessageReply{TextResponse: time.Now().String()}, nil
}

node.RegisterFunc("Time", f)
```

Function need to have the signature
```go
type HandleFunc func(msg *shared.GossipMessage) (*shared.MessageReply, error)
```

## Step 4: Binding to a listener

After registering the functions, the node is ready to be bound to a port. This is done by creating a `net.Listener` and passing it 
as a parameter to the `Serve` method of the node. The node is activated right after a call to the `Serve` method.

```go
ln, err := net.Listen("tcp4", "127.0.0.1:5000")
node.Serve(ln)
```

### Step 5: Ready to Gossiping
A gossip message is defined as followings. In the current implementation, it just takes a string as a payload. However, implementation
of a more generic payload of a byte slice is under progress. 
```go

// declaring the payload
msgRequest := shared.MessageRequest{Text: "test"}


// filling up the gossip message with the payload and other meta data
msg := &shared.GossipMessage{
	Uuid:       1,
  // You will need to specify the name of function you which the receiver to use on this gossip message
	Method:     "Time",
  // Recipients includes an array of peer addressees to be contacted, they are usually other gossip nodes.
	Recipients: peers,
  // Payload to be carried by the gossip message
	Payload:    &shared.GossipMessage_MessageRequest{MessageRequest: &msgRequest},
} 
```

After creating the gossip message, the `Gossip` method is utilized to gossip it within the network. 
There are two modes of sending Gossip messages; synchronously or asynchronously

| Name | Definition |
| ------------- | ------------- |
| asynchronous | The thread that invokes `Gossip` continuous write after the delivery of the message|
| synchronous  | The thread that invokes `Gossip` gets blocked until the message gets processed at the callee. The thread then gets the result back from the callee and returns |


The `Gossip` method takes a context and a gossip message as follows:
```go
// Gossip by default uses async calls. It will not block and will return a void message and an error
_, err := node.Gossip(context.Background(), msg)
```

The default mode of the node is asynchronous. The mode can be switched between synchronous and asynchronous by using the `SetSync` method:
```go
// switching to synchronous
node.SetSync(true)

// New gossip calls will now block until the message gets processed by peers and
// then returns an array of responses and an error
resp, err := node.Gossip(context.Background(), msg)
```
```go
//switching to asynchronous
node.SetSync(false)
```
