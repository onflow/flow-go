# bamboo-emulator

## Getting Started

```bash
> docker build -t bamboo-emulator .
> docker run -p 5000:5000 -it bamboo-emulator
```

### Submitting Transactions via gRPC
TODO: update this later
```bash
> prototool grpc services --address 0.0.0.0:5000 --method bamboo.services.access.v1.BambooAccessAPI/SendTransaction --data '{"transaction":{ "nonce": 1}}'
```