# Token Counting Service

Token-counting provides a gRPC endpoint for counting tokens in OpenAI
Responses API message payloads, including image inputs.

## Build and Test

```sh
make proto
make test
make build
```

## Configuration

| Variable | Default | Description |
| --- | --- | --- |
| `GRPC_ADDR` | `:50051` | gRPC listen address |
| `LOG_LEVEL` | `info` | Log level (`debug`, `info`, `warn`, `error`) |
