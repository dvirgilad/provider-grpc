# Crossplane GRPC Provider

A Crossplane provider that enables making arbitrary GRPC calls to any GRPC service using dynamic protobuf schemas.

## Features

- **Dynamic Protobuf Support**: Load protobuf schemas from Secrets, ConfigMaps, or inline definitions
- **Flexible GRPC Connections**: Support for TLS, custom headers, and timeouts
- **JSON Request/Response**: Use JSON to define request data and receive response data
- **Crossplane Integration**: Full lifecycle management as Crossplane managed resources
- **Multiple Provider Configs**: Support different GRPC servers with different protobuf schemas

## Quick Start

### 1. Install the Provider

```bash
# Install the provider (when published)
kubectl apply -f provider.yaml
```

### 2. Create a ProviderConfig

```yaml
apiVersion: template.crossplane.io/v1alpha1
kind: ProviderConfig
metadata:
  name: my-grpc-config
spec:
  credentials:
    source: None
  grpcServerConfig:
    host: "localhost"
    port: 50051
    timeout: 30
    tls:
      enabled: false
  protobufConfig:
    source: Inline
    inline: |
      syntax = "proto3";
      
      package helloworld;
      
      service Greeter {
        rpc SayHello (HelloRequest) returns (HelloReply) {}
      }
      
      message HelloRequest {
        string name = 1;
      }
      
      message HelloReply {
        string message = 1;
      }
```

### 3. Create a GRPC Call

```yaml
apiVersion: grpc.template.crossplane.io/v1alpha1
kind: GrpcCall
metadata:
  name: hello-call
  namespace: default
spec:
  forProvider:
    serviceName: "helloworld.Greeter"
    methodName: "SayHello"
    requestData:
      name: "World"
    headers:
      user-agent: "crossplane-grpc-provider/1.0"
    timeout: 10
  providerConfigRef:
    name: my-grpc-config
```

## Configuration Reference

### ProviderConfig

The `ProviderConfig` defines how to connect to a GRPC server and where to load the protobuf schema.

#### GRPC Server Configuration

```yaml
grpcServerConfig:
  host: "grpc.example.com"    # GRPC server hostname/IP
  port: 443                   # GRPC server port
  timeout: 30                 # Default timeout in seconds
  tls:
    enabled: true             # Enable/disable TLS
    serverName: "grpc.example.com"  # TLS server name
    insecureSkipVerify: false # Skip TLS verification (for testing)
```

#### Protobuf Configuration

**Inline Protobuf:**
```yaml
protobufConfig:
  source: Inline
  inline: |
    syntax = "proto3";
    // your protobuf definition here
```

**From Secret:**
```yaml
protobufConfig:
  source: Secret
  secretRef:
    name: my-proto-secret
    key: service.proto
    namespace: default
```

**From ConfigMap:**
```yaml
protobufConfig:
  source: ConfigMap
  configMapRef:
    name: my-proto-configmap
    key: service.proto
    namespace: default
```

### GrpcCall

The `GrpcCall` resource defines a specific GRPC method call.

```yaml
apiVersion: grpc.template.crossplane.io/v1alpha1
kind: GrpcCall
metadata:
  name: my-call
  namespace: default
spec:
  forProvider:
    serviceName: "package.ServiceName"  # Fully qualified service name
    methodName: "MethodName"            # Method to call
    requestData:                        # JSON request data
      field1: "value1"
      field2: 42
      field3:
        nestedField: true
    headers:                            # Optional GRPC headers
      authorization: "Bearer token"
      custom-header: "value"
    timeout: 15                         # Optional call timeout (overrides provider config)
  providerConfigRef:
    name: my-grpc-config
```

### Response Data

After a successful GRPC call, the response data is available in the resource status:

```bash
kubectl get grpccall my-call -o yaml
```

```yaml
status:
  atProvider:
    responseData:
      message: "Hello, World!"
    responseHeaders:
      content-type: "application/grpc"
    lastCallTime: "2024-01-15T10:30:00Z"
    callCount: 1
  conditions:
  - type: Ready
    status: "True"
  - type: Synced
    status: "True"
```

## Examples

See the [examples](./examples/grpc/) directory for complete working examples:

- **Basic Hello World**: Simple GRPC call with inline protobuf
- **TLS Configuration**: Secure GRPC connection with TLS
- **Complex Service**: Inventory service with CRUD operations
- **Secret-based Protobuf**: Loading protobuf from Kubernetes secrets

## Development

### Building

```bash
make submodules
make reviewable
make build
```

### Testing

```bash
make test
```

### Running Locally

```bash
make run
```

## Security Considerations

- **TLS**: Always use TLS in production environments
- **Credentials**: Store sensitive data (API keys, certificates) in Kubernetes secrets
- **Network Policies**: Restrict network access to GRPC servers
- **RBAC**: Use appropriate Kubernetes RBAC for provider resources

## Limitations

- Currently uses deprecated protoreflect libraries (will be updated to newer versions)
- Protobuf file parsing is simplified (may not handle all complex protobuf features)
- No support for streaming GRPC calls (unary calls only)
- No built-in retry mechanism (relies on Crossplane's reconciliation)

## Contributing

Contributions are welcome! Please read the [contributing guide](./CONTRIBUTING.md) for details.

## License

This project is licensed under the Apache License 2.0 - see the [LICENSE](./LICENSE) file for details.