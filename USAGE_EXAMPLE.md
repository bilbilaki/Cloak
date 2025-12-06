# Cloak Wrapper Usage Examples

This document provides practical examples of using the Cloak C-shared wrappers with Dart FFI.

## Table of Contents

- [Client Example](#client-example)
- [Server Example](#server-example)
- [Common Patterns](#common-patterns)

## Client Example

### Complete Dart Client Integration

```dart
import 'dart:ffi' as ffi;
import 'dart:isolate';
import 'dart:convert';
import 'package:ffi/ffi.dart';

class CloakClient {
  late final DynamicLibrary _lib;
  late final ReceivePort _receivePort;
  int? _currentTaskId;
  
  // Function signatures
  late final void Function(ffi.Pointer<ffi.Void>) _bridgeInit;
  late final void Function(int) _registerPort;
  late final void Function() _unregisterPort;
  late final int Function(ffi.Pointer<Utf8>, int, int) _startCloakClient;
  late final void Function(int) _generateUID;
  late final void Function(int) _getVersion;
  late final void Function(int, int) _stopTask;
  
  CloakClient(String libraryPath) {
    _lib = DynamicLibrary.open(libraryPath);
    _bindFunctions();
  }
  
  void _bindFunctions() {
    _bridgeInit = _lib.lookupFunction<
      ffi.Void Function(ffi.Pointer<ffi.Void>),
      void Function(ffi.Pointer<ffi.Void>)
    >('BridgeInit');
    
    _registerPort = _lib.lookupFunction<
      ffi.Void Function(ffi.Int64),
      void Function(int)
    >('RegisterPort');
    
    _unregisterPort = _lib.lookupFunction<
      ffi.Void Function(),
      void Function()
    >('UnregisterPort');
    
    _startCloakClient = _lib.lookupFunction<
      ffi.Int64 Function(ffi.Pointer<Utf8>, ffi.Int, ffi.Int64),
      int Function(ffi.Pointer<Utf8>, int, int)
    >('StartCloakClient');
    
    _generateUID = _lib.lookupFunction<
      ffi.Void Function(ffi.Int64),
      void Function(int)
    >('GenerateUID');
    
    _getVersion = _lib.lookupFunction<
      ffi.Void Function(ffi.Int64),
      void Function(int)
    >('GetVersion');
    
    _stopTask = _lib.lookupFunction<
      ffi.Void Function(ffi.Int64, ffi.Int64),
      void Function(int, int)
    >('StopTask');
  }
  
  Future<void> initialize() async {
    // Initialize Dart API
    final initApi = ffi.NativeApi.initializeApiDLData;
    _bridgeInit(initApi);
    
    // Setup port for receiving responses
    _receivePort = ReceivePort();
    _receivePort.listen(_handleResponse);
    _registerPort(_receivePort.sendPort.nativePort);
  }
  
  void _handleResponse(dynamic message) {
    if (message is String) {
      try {
        final json = jsonDecode(message);
        final op = json['op'];
        final success = json['success'];
        final error = json['error'];
        final data = json['data'];
        
        print('Operation: $op');
        print('Success: $success');
        if (!success) {
          print('Error: $error');
        } else {
          print('Data: $data');
        }
        
        // Handle specific operations
        switch (op) {
          case 'start_cloak_client':
            if (success) {
              _currentTaskId = data;
              print('Client started with task ID: $_currentTaskId');
            }
            break;
          case 'generate_uid':
            if (success) {
              print('Generated UID: $data');
            }
            break;
          case 'get_version':
            if (success) {
              print('Version: $data');
            }
            break;
        }
      } catch (e) {
        print('Failed to parse response: $e');
      }
    }
  }
  
  void startClient(Map<String, dynamic> config, {bool udp = false}) {
    final configJson = jsonEncode(config);
    final configPtr = configJson.toNativeUtf8();
    
    try {
      final taskId = _startCloakClient(
        configPtr,
        udp ? 1 : 0,
        _receivePort.sendPort.nativePort
      );
      print('Start client request sent, got task ID: $taskId');
    } finally {
      malloc.free(configPtr);
    }
  }
  
  void generateUID() {
    _generateUID(_receivePort.sendPort.nativePort);
  }
  
  void getVersion() {
    _getVersion(_receivePort.sendPort.nativePort);
  }
  
  void stop() {
    if (_currentTaskId != null) {
      _stopTask(_currentTaskId!, _receivePort.sendPort.nativePort);
      _currentTaskId = null;
    }
  }
  
  void dispose() {
    stop();
    _unregisterPort();
    _receivePort.close();
  }
}

// Usage
void main() async {
  final client = CloakClient('./build/libcloak_client.so');
  await client.initialize();
  
  // Get version
  client.getVersion();
  
  // Generate UID
  client.generateUID();
  
  // Start client with configuration
  final config = {
    'ServerName': 'example.com',
    'ProxyMethod': 'shadowsocks',
    'EncryptionMethod': 'aes-256-gcm',
    'UID': 'your-base64-uid',
    'PublicKey': 'your-base64-public-key',
    'NumConn': 4,
    'LocalHost': '127.0.0.1',
    'LocalPort': '1984',
    'RemoteHost': 'server.example.com',
    'RemotePort': '443',
    'Transport': 'direct',
    'BrowserSig': 'chrome'
  };
  
  client.startClient(config, udp: false);
  
  // Keep running...
  await Future.delayed(Duration(seconds: 60));
  
  // Stop
  client.dispose();
}
```

## Server Example

### Complete Dart Server Integration

```dart
import 'dart:ffi' as ffi;
import 'dart:isolate';
import 'dart:convert';
import 'package:ffi/ffi.dart';

class CloakServer {
  late final DynamicLibrary _lib;
  late final ReceivePort _receivePort;
  int? _currentTaskId;
  
  // Function signatures
  late final void Function(ffi.Pointer<ffi.Void>) _bridgeInit;
  late final void Function(int) _registerPort;
  late final void Function() _unregisterPort;
  late final int Function(ffi.Pointer<Utf8>, int) _startCloakServer;
  late final void Function(int) _generateKeyPair;
  late final void Function(int) _generateUID;
  late final void Function(int) _getVersion;
  late final void Function(int, int) _stopTask;
  
  CloakServer(String libraryPath) {
    _lib = DynamicLibrary.open(libraryPath);
    _bindFunctions();
  }
  
  void _bindFunctions() {
    _bridgeInit = _lib.lookupFunction<
      ffi.Void Function(ffi.Pointer<ffi.Void>),
      void Function(ffi.Pointer<ffi.Void>)
    >('BridgeInit');
    
    _registerPort = _lib.lookupFunction<
      ffi.Void Function(ffi.Int64),
      void Function(int)
    >('RegisterPort');
    
    _unregisterPort = _lib.lookupFunction<
      ffi.Void Function(),
      void Function()
    >('UnregisterPort');
    
    _startCloakServer = _lib.lookupFunction<
      ffi.Int64 Function(ffi.Pointer<Utf8>, ffi.Int64),
      int Function(ffi.Pointer<Utf8>, int)
    >('StartCloakServer');
    
    _generateKeyPair = _lib.lookupFunction<
      ffi.Void Function(ffi.Int64),
      void Function(int)
    >('GenerateKeyPair');
    
    _generateUID = _lib.lookupFunction<
      ffi.Void Function(ffi.Int64),
      void Function(int)
    >('GenerateUID');
    
    _getVersion = _lib.lookupFunction<
      ffi.Void Function(ffi.Int64),
      void Function(int)
    >('GetVersion');
    
    _stopTask = _lib.lookupFunction<
      ffi.Void Function(ffi.Int64, ffi.Int64),
      void Function(int, int)
    >('StopTask');
  }
  
  Future<void> initialize() async {
    final initApi = ffi.NativeApi.initializeApiDLData;
    _bridgeInit(initApi);
    
    _receivePort = ReceivePort();
    _receivePort.listen(_handleResponse);
    _registerPort(_receivePort.sendPort.nativePort);
  }
  
  void _handleResponse(dynamic message) {
    if (message is String) {
      try {
        final json = jsonDecode(message);
        final op = json['op'];
        final success = json['success'];
        final error = json['error'];
        final data = json['data'];
        
        print('Operation: $op');
        print('Success: $success');
        if (!success) {
          print('Error: $error');
        } else {
          print('Data: $data');
        }
        
        switch (op) {
          case 'start_cloak_server':
            if (success) {
              _currentTaskId = data;
              print('Server started with task ID: $_currentTaskId');
            }
            break;
          case 'generate_keypair':
            if (success) {
              print('Generated key pair:');
              print('  Public Key: ${data['publicKey']}');
              print('  Private Key: ${data['privateKey']}');
            }
            break;
          case 'generate_uid':
            if (success) {
              print('Generated UID: $data');
            }
            break;
        }
      } catch (e) {
        print('Failed to parse response: $e');
      }
    }
  }
  
  void startServer(Map<String, dynamic> config) {
    final configJson = jsonEncode(config);
    final configPtr = configJson.toNativeUtf8();
    
    try {
      final taskId = _startCloakServer(
        configPtr,
        _receivePort.sendPort.nativePort
      );
      print('Start server request sent, got task ID: $taskId');
    } finally {
      malloc.free(configPtr);
    }
  }
  
  void generateKeyPair() {
    _generateKeyPair(_receivePort.sendPort.nativePort);
  }
  
  void generateUID() {
    _generateUID(_receivePort.sendPort.nativePort);
  }
  
  void getVersion() {
    _getVersion(_receivePort.sendPort.nativePort);
  }
  
  void stop() {
    if (_currentTaskId != null) {
      _stopTask(_currentTaskId!, _receivePort.sendPort.nativePort);
      _currentTaskId = null;
    }
  }
  
  void dispose() {
    stop();
    _unregisterPort();
    _receivePort.close();
  }
}

// Usage
void main() async {
  final server = CloakServer('./build/libcloak_server.so');
  await server.initialize();
  
  // Generate keys and UIDs
  server.generateKeyPair();
  server.generateUID();
  server.getVersion();
  
  await Future.delayed(Duration(seconds: 2));
  
  // Start server with configuration
  final config = {
    'ProxyBook': {
      'shadowsocks': ['tcp', '127.0.0.1:8388']
    },
    'BindAddr': [':443'],
    'BypassUID': [],
    'RedirAddr': 'www.example.com',
    'PrivateKey': 'your-base64-private-key',
    'AdminUID': '',
    'DatabasePath': '',
    'KeepAlive': 30
  };
  
  server.startServer(config);
  
  // Keep running...
  await Future.delayed(Duration(hours: 1));
  
  // Stop
  server.dispose();
}
```

## Common Patterns

### Error Handling

```dart
void _handleResponse(dynamic message) {
  if (message is String) {
    try {
      final json = jsonDecode(message);
      
      if (!json['success']) {
        // Handle error
        print('Error in ${json['op']}: ${json['error']}');
        
        // You might want to retry or notify the user
        if (json['op'] == 'start_cloak_client') {
          // Retry connection logic
        }
      }
    } catch (e) {
      print('Failed to parse response: $e');
    }
  }
}
```

### Async/Await Pattern

```dart
Future<String> generateUIDAsync() async {
  final completer = Completer<String>();
  
  late StreamSubscription subscription;
  subscription = _receivePort.listen((message) {
    if (message is String) {
      final json = jsonDecode(message);
      if (json['op'] == 'generate_uid') {
        subscription.cancel();
        if (json['success']) {
          completer.complete(json['data']);
        } else {
          completer.completeError(json['error']);
        }
      }
    }
  });
  
  _generateUID(_receivePort.sendPort.nativePort);
  return completer.future;
}
```

### Configuration Management

```dart
class CloakConfig {
  static Map<String, dynamic> defaultClientConfig() {
    return {
      'ServerName': 'example.com',
      'ProxyMethod': 'shadowsocks',
      'EncryptionMethod': 'aes-256-gcm',
      'NumConn': 4,
      'LocalHost': '127.0.0.1',
      'LocalPort': '1984',
      'RemotePort': '443',
      'Transport': 'direct',
      'BrowserSig': 'chrome',
      'StreamTimeout': 300,
      'KeepAlive': 0
    };
  }
  
  static Map<String, dynamic> defaultServerConfig() {
    return {
      'BindAddr': [':443'],
      'BypassUID': [],
      'RedirAddr': 'www.example.com',
      'KeepAlive': 30,
      'CncMode': false
    };
  }
}
```

### Memory Management

```dart
// Always free C strings after use
void callWithString(String str) {
  final cStr = str.toNativeUtf8();
  try {
    // Use cStr
    _someFunction(cStr);
  } finally {
    // Always free to prevent memory leaks
    malloc.free(cStr);
  }
}
```

## Notes

1. **Thread Safety**: All wrapper functions are thread-safe and can be called from different isolates.

2. **Long-Running Operations**: Use task IDs to manage long-running operations like client/server instances.

3. **Cleanup**: Always call `dispose()` or equivalent cleanup methods to properly release resources.

4. **JSON Responses**: All responses are JSON-formatted strings that need to be parsed.

5. **Port Communication**: The bridge uses Dart ports for asynchronous communication between Go and Dart.

6. **Error Recovery**: Implement proper error handling and retry logic for network operations.

## Dependencies

Add these to your `pubspec.yaml`:

```yaml
dependencies:
  ffi: ^2.0.0
```

## Building for Different Platforms

### Linux
```bash
CGO_ENABLED=1 GOOS=linux GOARCH=amd64 make wrappers
```

### Android (ARM64)
```bash
CGO_ENABLED=1 GOOS=android GOARCH=arm64 \
  CC=$NDK_PATH/toolchains/llvm/prebuilt/linux-x86_64/bin/aarch64-linux-android30-clang \
  make wrappers
```

### iOS (ARM64)
```bash
CGO_ENABLED=1 GOOS=ios GOARCH=arm64 \
  SDK=iphoneos \
  make wrappers
```
