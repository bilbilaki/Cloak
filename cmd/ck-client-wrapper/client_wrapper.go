package main

/*
#include <stdint.h>
*/
import "C"
import (
	"context"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"unsafe"

	bridge "github.com/bilbilaki/Cloak/hiddify-libclash/bridge"
	"github.com/cbeuw/Cloak/internal/client"
	"github.com/cbeuw/Cloak/internal/common"
	mux "github.com/cbeuw/Cloak/internal/multiplex"
)

const (
	Version = "v1.0.0, Cloak Client Wrapper"
)

var (
	registeredPort int64
	portMu         sync.RWMutex
)

var (
	tasks    = make(map[int64]*taskRec)
	tasksMu  sync.Mutex
	nextTask int64
)

type taskRec struct {
	cancel context.CancelFunc
	done   chan struct{}
}

type simpleResp struct {
	Op      string      `json:"op"`
	Success bool        `json:"success"`
	Error   string      `json:"error,omitempty"`
	Data    interface{} `json:"data,omitempty"`
}

func addTask(cancel context.CancelFunc) int64 {
	id := atomic.AddInt64(&nextTask, 1)
	entry := &taskRec{
		cancel: cancel,
		done:   make(chan struct{}),
	}
	tasksMu.Lock()
	tasks[id] = entry
	tasksMu.Unlock()
	return id
}

func finishTask(id int64) {
	tasksMu.Lock()
	if e, ok := tasks[id]; ok {
		close(e.done)
		delete(tasks, id)
	}
	tasksMu.Unlock()
}

//export RegisterPort
func RegisterPort(port C.longlong) {
	portMu.Lock()
	registeredPort = int64(port)
	portMu.Unlock()
	bridge.SendStringToPort(int64(port), "registered")
}

//export UnregisterPort
func UnregisterPort() {
	portMu.Lock()
	p := registeredPort
	registeredPort = 0
	portMu.Unlock()
	if p != 0 {
		bridge.SendStringToPort(p, "unregistered")
	}
}

func getPortOrDefault(p C.longlong) int64 {
	if int64(p) != 0 {
		return int64(p)
	}
	portMu.RLock()
	defer portMu.RUnlock()
	return registeredPort
}

func sendToPort(port int64, r simpleResp) {
	b, _ := json.Marshal(r)
	bridge.SendStringToPort(port, string(b))
}

func safeOp(port int64, op string, fn func() (interface{}, error)) {
	defer func() {
		if r := recover(); r != nil {
			sendToPort(port, simpleResp{Op: op, Success: false, Error: fmt.Sprintf("panic: %v", r)})
		}
	}()
	res, err := fn()
	if err != nil {
		sendToPort(port, simpleResp{Op: op, Success: false, Error: err.Error()})
		return
	}
	sendToPort(port, simpleResp{Op: op, Success: true, Data: res})
}

//export BridgeInit
func BridgeInit(api unsafe.Pointer) {
	defer func() {
		if r := recover(); r != nil {
			port := getPortOrDefault(0)
			if port != 0 {
				sendToPort(port, simpleResp{Op: "bridge_init", Success: false, Error: fmt.Sprintf("%v", r)})
			}
		}
	}()
	bridge.InitDartApi(api)
}

//export StartCloakClient
func StartCloakClient(configJSON *C.char, udp C.int, port C.longlong) C.longlong {
	p := getPortOrDefault(port)
	var taskID int64

	ctx, cancel := context.WithCancel(context.Background())
	taskID = addTask(cancel)

	goConfigJSON := C.GoString(configJSON)

	go func(tid int64) {
		defer finishTask(tid)
		defer func() {
			if r := recover(); r != nil {
				sendToPort(p, simpleResp{Op: "start_cloak_client", Success: false, Error: fmt.Sprintf("panic: %v", r)})
			}
		}()

		// Parse the config
		var rawConfig client.RawConfig
		err := json.Unmarshal([]byte(goConfigJSON), &rawConfig)
		if err != nil {
			sendToPort(p, simpleResp{Op: "start_cloak_client", Success: false, Error: fmt.Sprintf("failed to parse config: %v", err)})
			return
		}

		// Process the config
		localConfig, remoteConfig, authInfo, err := rawConfig.ProcessRawConfig(common.RealWorldState)
		if err != nil {
			sendToPort(p, simpleResp{Op: "start_cloak_client", Success: false, Error: fmt.Sprintf("failed to process config: %v", err)})
			return
		}

		// Create dialer
		dialer := &net.Dialer{KeepAlive: remoteConfig.KeepAlive}

		// Create session function
		newSeshFunc := func() *mux.Session {
			authInfo := authInfo // copy the struct because we are overwriting SessionId

			randByte := make([]byte, 1)
			common.RandRead(authInfo.WorldState.Rand, randByte)
			authInfo.MockDomain = localConfig.MockDomainList[int(randByte[0])%len(localConfig.MockDomainList)]

			// sessionID is usergenerated
			quad := make([]byte, 4)
			common.RandRead(authInfo.WorldState.Rand, quad)
			authInfo.SessionId = binary.BigEndian.Uint32(quad)
			return client.MakeSession(remoteConfig, authInfo, dialer)
		}

		// Start routing based on UDP or TCP mode
		if int(udp) == 1 {
			// UDP mode
			bindFunc := func() (*net.UDPConn, error) {
				localAddr, err := net.ResolveUDPAddr("udp", localConfig.LocalAddr)
				if err != nil {
					return nil, err
				}
				return net.ListenUDP("udp", localAddr)
			}
			
			sendToPort(p, simpleResp{Op: "start_cloak_client", Success: true, Data: tid})
			client.RouteUDP(bindFunc, localConfig.Timeout, remoteConfig.Singleplex, newSeshFunc)
		} else {
			// TCP mode
			listener, err := net.Listen("tcp", localConfig.LocalAddr)
			if err != nil {
				sendToPort(p, simpleResp{Op: "start_cloak_client", Success: false, Error: fmt.Sprintf("failed to listen: %v", err)})
				return
			}
			defer listener.Close()

			sendToPort(p, simpleResp{Op: "start_cloak_client", Success: true, Data: tid})
			client.RouteTCP(listener, localConfig.Timeout, remoteConfig.Singleplex, newSeshFunc)
		}

		<-ctx.Done()
	}(taskID)

	return C.longlong(taskID)
}

//export GenerateUID
func GenerateUID(port C.longlong) {
	p := getPortOrDefault(port)
	safeOp(p, "generate_uid", func() (interface{}, error) {
		UID := [16]byte{}
		common.RandRead(common.RealWorldState.Rand, UID[:])
		return base64.StdEncoding.EncodeToString(UID[:]), nil
	})
}

//export GetVersion
func GetVersion(port C.longlong) {
	p := getPortOrDefault(port)
	sendToPort(p, simpleResp{Op: "get_version", Success: true, Data: Version})
}

//export StopTask
func StopTask(taskID C.longlong, port C.longlong) {
	id := int64(taskID)
	p := getPortOrDefault(port)

	tasksMu.Lock()
	entry, ok := tasks[id]
	if ok {
		delete(tasks, id)
	}
	tasksMu.Unlock()

	if !ok {
		sendToPort(p, simpleResp{Op: "stop_task", Success: false, Error: fmt.Sprintf("task %d not found", id)})
		return
	}

	entry.cancel()
	<-entry.done
	sendToPort(p, simpleResp{Op: "stop_task", Success: true, Data: id})
}

func main() {}
