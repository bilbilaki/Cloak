package main

/*
#include <stdint.h>
*/
import "C"
import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"unsafe"

	bridge "github.com/bilbilaki/Cloak/hiddify-libclash/bridge"
	"github.com/cbeuw/Cloak/internal/common"
	"github.com/cbeuw/Cloak/internal/ecdh"
	"github.com/cbeuw/Cloak/internal/server"
	"crypto/rand"
)

const (
	Version = "v1.0.0, Cloak Server Wrapper"
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

//export StartCloakServer
func StartCloakServer(configJSON *C.char, port C.longlong) C.longlong {
	p := getPortOrDefault(port)
	var taskID int64

	ctx, cancel := context.WithCancel(context.Background())
	taskID = addTask(cancel)

	goConfigJSON := C.GoString(configJSON)

	go func(tid int64) {
		defer finishTask(tid)
		defer func() {
			if r := recover(); r != nil {
				sendToPort(p, simpleResp{Op: "start_cloak_server", Success: false, Error: fmt.Sprintf("panic: %v", r)})
			}
		}()

		// Parse the config
		var rawConfig server.RawConfig
		err := json.Unmarshal([]byte(goConfigJSON), &rawConfig)
		if err != nil {
			sendToPort(p, simpleResp{Op: "start_cloak_server", Success: false, Error: fmt.Sprintf("failed to parse config: %v", err)})
			return
		}

		// Initialize the server state
		sta, err := server.InitState(rawConfig, common.RealWorldState)
		if err != nil {
			sendToPort(p, simpleResp{Op: "start_cloak_server", Success: false, Error: fmt.Sprintf("failed to initialize server: %v", err)})
			return
		}

		// Parse bind addresses
		bindAddrs := rawConfig.BindAddr
		if len(bindAddrs) == 0 {
			bindAddrs = []string{":443"}
		}

		var listeners []net.Listener
		for _, addr := range bindAddrs {
			listener, err := net.Listen("tcp", addr)
			if err != nil {
				sendToPort(p, simpleResp{Op: "start_cloak_server", Success: false, Error: fmt.Sprintf("failed to bind %v: %v", addr, err)})
				return
			}
			listeners = append(listeners, listener)
		}

		// Start dispatcher for each listener
		for _, listener := range listeners {
			go server.Serve(listener, sta)
		}

		sendToPort(p, simpleResp{Op: "start_cloak_server", Success: true, Data: tid})
		<-ctx.Done()

		// Clean up listeners
		for _, listener := range listeners {
			listener.Close()
		}
	}(taskID)

	return C.longlong(taskID)
}

//export GenerateKeyPair
func GenerateKeyPair(port C.longlong) {
	p := getPortOrDefault(port)
	safeOp(p, "generate_keypair", func() (interface{}, error) {
		staticPv, staticPub, err := ecdh.GenerateKey(rand.Reader)
		if err != nil {
			return nil, err
		}
		
		marshPub := ecdh.Marshal(staticPub)
		marshPv := staticPv.(*[32]byte)[:]
		
		result := map[string]string{
			"publicKey":  base64.StdEncoding.EncodeToString(marshPub),
			"privateKey": base64.StdEncoding.EncodeToString(marshPv),
		}
		return result, nil
	})
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
