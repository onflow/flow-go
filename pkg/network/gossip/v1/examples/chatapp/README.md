# Chatapp
A simple group chat made with the gossip network layer. It also demonstrates the registration of gRPC services by user into the Gossip registry using gsip.

## How to run
Go to `pkg/network/gossip/v1/examples/chatapp`
and run the following:

### Build
```bash
go build -o chatapp .
```

### Run
Run three instances of the chatapp
```
./chatapp #(in three different terminals, but one is enough)
```

In each of those terminals you can start sending messages to other chatapp
instances

```
Enter Message: Hello, world!
```
