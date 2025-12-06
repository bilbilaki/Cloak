# Cloak C-Shared Wrappers for Dart FFI

This directory contains C-shared library wrappers for both Cloak client and server, enabling integration with Dart applications through FFI (Foreign Function Interface).

## Overview

The wrappers provide a bridge between Cloak's Go implementation and Dart applications, using the hiddify-libclash bridge for communication.

## Structure

```
.
├── cmd/
│   ├── ck-client-wrapper/   # Client wrapper source
│   └── ck-server-wrapper/   # Server wrapper source
├── hiddify-libclash/        # Git submodule containing the bridge
└── build/                    # Compiled libraries
    ├── libcloak_client.so   # Client shared library
    ├── libcloak_client.h    # Client header file
    ├── libcloak_server.so   # Server shared library
    └── libcloak_server.h    # Server header file
```

## Building

### Prerequisites

- Go 1.24.0 or later
- CGO enabled
- GCC or compatible C compiler

### Build Commands

Build both wrappers:
```bash
make wrappers
```

Build client wrapper only:
```bash
make wrapper-client
```

Build server wrapper only:
```bash
make wrapper-server
```

Clean build artifacts:
```bash
make clean
```

## Client Wrapper API

### Exported Functions

- **`BridgeInit(api unsafe.Pointer)`** - Initialize the Dart API bridge
- **`RegisterPort(port int64)`** - Register a Dart port for communication
- **`UnregisterPort()`** - Unregister the current Dart port
- **`StartCloakClient(configJSON *char, udp int, port int64) int64`** - Start a Cloak client
  - `configJSON`: JSON configuration string
  - `udp`: 1 for UDP mode, 0 for TCP mode
  - `port`: Dart port for responses
  - Returns: Task ID
- **`GenerateUID(port int64)`** - Generate a new user ID
- **`GetVersion(port int64)`** - Get wrapper version
- **`StopTask(taskID int64, port int64)`** - Stop a running task

### Client Configuration Example

```json
{
  "ServerName": "example.com",
  "ProxyMethod": "shadowsocks",
  "EncryptionMethod": "aes-256-gcm",
  "UID": "base64-encoded-uid",
  "PublicKey": "base64-encoded-public-key",
  "NumConn": 4,
  "LocalHost": "127.0.0.1",
  "LocalPort": "1984",
  "RemoteHost": "server.example.com",
  "RemotePort": "443",
  "Transport": "direct",
  "BrowserSig": "chrome"
}
```

## Server Wrapper API

### Exported Functions

- **`BridgeInit(api unsafe.Pointer)`** - Initialize the Dart API bridge
- **`RegisterPort(port int64)`** - Register a Dart port for communication
- **`UnregisterPort()`** - Unregister the current Dart port
- **`StartCloakServer(configJSON *char, port int64) int64`** - Start a Cloak server
  - `configJSON`: JSON configuration string
  - `port`: Dart port for responses
  - Returns: Task ID
- **`GenerateKeyPair(port int64)`** - Generate a new public/private key pair
- **`GenerateUID(port int64)`** - Generate a new user ID
- **`GetVersion(port int64)`** - Get wrapper version
- **`StopTask(taskID int64, port int64)`** - Stop a running task

### Server Configuration Example

```json
{
  "ProxyBook": {
    "shadowsocks": ["tcp", "127.0.0.1:8388"]
  },
  "BindAddr": [":443"],
  "BypassUID": ["base64-encoded-uid"],
  "RedirAddr": "www.example.com",
  "PrivateKey": "base64-encoded-private-key",
  "AdminUID": "base64-encoded-admin-uid",
  "DatabasePath": "/path/to/userinfo.db",
  "KeepAlive": 30
}
```

## Communication Protocol

Both wrappers use a JSON-based response format:

```json
{
  "op": "operation_name",
  "success": true,
  "error": "error message if any",
  "data": "operation-specific data"
}
```

Responses are sent to the registered Dart port using the bridge's `SendStringToPort` function.

## Usage in Dart

1. Load the shared library using `dart:ffi`
2. Initialize the Dart API with `BridgeInit`
3. Register a Dart port with `RegisterPort`
4. Call wrapper functions as needed
5. Handle JSON responses in Dart

Example Dart code structure:

```dart
import 'dart:ffi' as ffi;
import 'dart:isolate';

// Load the library
final DynamicLibrary lib = DynamicLibrary.open('libcloak_client.so');

// Define function signatures
typedef BridgeInitNative = ffi.Void Function(ffi.Pointer<ffi.Void> api);
typedef BridgeInitDart = void Function(ffi.Pointer<ffi.Void> api);

typedef RegisterPortNative = ffi.Void Function(ffi.Int64 port);
typedef RegisterPortDart = void Function(int port);

// Lookup functions
final bridgeInit = lib.lookupFunction<BridgeInitNative, BridgeInitDart>('BridgeInit');
final registerPort = lib.lookupFunction<RegisterPortNative, RegisterPortDart>('RegisterPort');

// Initialize
void initBridge() {
  final api = ffi.NativeApi.initializeApiDLData;
  bridgeInit(api);
  
  final receivePort = ReceivePort();
  registerPort(receivePort.sendPort.nativePort);
  
  receivePort.listen((message) {
    // Handle JSON responses
    print('Received: $message');
  });
}
```

## Thread Safety

- All exported functions are thread-safe
- Multiple concurrent operations are supported through task IDs
- Use unique task IDs to manage multiple operations

## Error Handling

All operations use the `safeOp` wrapper that:
- Catches panics and returns them as errors
- Validates input parameters
- Sends structured error responses

## Notes

- The bridge submodule must be initialized before building: `git submodule update --init --recursive`
- Both wrappers support long-running operations that can be stopped using `StopTask`
- Configuration is passed as JSON strings for flexibility
- All responses are asynchronous and sent to the registered Dart port

## License

This project inherits the license from the main Cloak project (see LICENSE file in the root directory).
