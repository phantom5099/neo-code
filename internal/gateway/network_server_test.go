package gateway

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"strings"
	"testing"
	"time"

	"golang.org/x/net/websocket"

	"neo-code/internal/gateway/protocol"
)

func TestResolveNetworkListenAddress(t *testing.T) {
	t.Run("default address", func(t *testing.T) {
		address, err := ResolveNetworkListenAddress("")
		if err != nil {
			t.Fatalf("resolve default address: %v", err)
		}
		if address != DefaultNetworkListenAddress {
			t.Fatalf("address = %q, want %q", address, DefaultNetworkListenAddress)
		}
	})

	t.Run("loopback accepted", func(t *testing.T) {
		address, err := ResolveNetworkListenAddress("127.0.0.1:19080")
		if err != nil {
			t.Fatalf("resolve loopback address: %v", err)
		}
		if address != "127.0.0.1:19080" {
			t.Fatalf("address = %q, want %q", address, "127.0.0.1:19080")
		}
	})

	t.Run("non loopback rejected", func(t *testing.T) {
		_, err := ResolveNetworkListenAddress("0.0.0.0:8080")
		if err == nil {
			t.Fatal("expected non-loopback address error")
		}
		if !strings.Contains(err.Error(), "host must be loopback") {
			t.Fatalf("error = %v, want loopback constraint", err)
		}
	})
}

func TestNetworkServerHTTPRPCAndCORS(t *testing.T) {
	server := newTestNetworkServer(t)
	testContext, cancel := context.WithCancel(context.Background())
	defer cancel()

	serveDone := make(chan error, 1)
	go func() {
		serveDone <- server.Serve(testContext, nil)
	}()
	t.Cleanup(func() {
		_ = server.Close(context.Background())
		select {
		case <-serveDone:
		case <-time.After(2 * time.Second):
			t.Fatal("network serve goroutine did not exit")
		}
	})

	listenAddress := waitForNetworkAddress(t, server)

	requestBody := strings.NewReader(`{"jsonrpc":"2.0","id":"http-1","method":"gateway.ping","params":{}}`)
	response, err := http.Post("http://"+listenAddress+"/rpc", "application/json", requestBody)
	if err != nil {
		t.Fatalf("post /rpc: %v", err)
	}
	defer response.Body.Close()

	if response.Header.Get("Access-Control-Allow-Origin") != "*" {
		t.Fatalf("cors allow origin = %q, want %q", response.Header.Get("Access-Control-Allow-Origin"), "*")
	}

	var rpcResponse protocol.JSONRPCResponse
	if err := json.NewDecoder(response.Body).Decode(&rpcResponse); err != nil {
		t.Fatalf("decode /rpc response: %v", err)
	}
	if rpcResponse.Error != nil {
		t.Fatalf("unexpected rpc error: %+v", rpcResponse.Error)
	}
	resultFrame, err := decodeJSONRPCResultFrame(rpcResponse)
	if err != nil {
		t.Fatalf("decode result frame: %v", err)
	}
	if resultFrame.Type != FrameTypeAck || resultFrame.Action != FrameActionPing {
		t.Fatalf("result frame = %#v, want ping ack", resultFrame)
	}

	optionsRequest, err := http.NewRequest(http.MethodOptions, "http://"+listenAddress+"/rpc", nil)
	if err != nil {
		t.Fatalf("new options request: %v", err)
	}
	optionsResponse, err := http.DefaultClient.Do(optionsRequest)
	if err != nil {
		t.Fatalf("options /rpc: %v", err)
	}
	defer optionsResponse.Body.Close()

	if optionsResponse.StatusCode != http.StatusNoContent {
		t.Fatalf("options status = %d, want %d", optionsResponse.StatusCode, http.StatusNoContent)
	}
	if optionsResponse.Header.Get("Access-Control-Allow-Methods") == "" {
		t.Fatal("expected CORS allow methods header on OPTIONS response")
	}
}

func TestNetworkServerWebSocketAndSSEPing(t *testing.T) {
	server := newTestNetworkServer(t)
	testContext, cancel := context.WithCancel(context.Background())
	defer cancel()

	serveDone := make(chan error, 1)
	go func() {
		serveDone <- server.Serve(testContext, nil)
	}()
	t.Cleanup(func() {
		_ = server.Close(context.Background())
		select {
		case <-serveDone:
		case <-time.After(2 * time.Second):
			t.Fatal("network serve goroutine did not exit")
		}
	})

	listenAddress := waitForNetworkAddress(t, server)
	wsURL := "ws://" + listenAddress + "/ws"
	wsConn, err := websocket.Dial(wsURL, "", "http://localhost/")
	if err != nil {
		t.Fatalf("dial websocket: %v", err)
	}
	t.Cleanup(func() { _ = wsConn.Close() })

	if err := websocket.Message.Send(wsConn, `{"jsonrpc":"2.0","id":"ws-1","method":"gateway.ping","params":{}}`); err != nil {
		t.Fatalf("send websocket ping request: %v", err)
	}

	_ = wsConn.SetReadDeadline(time.Now().Add(2 * time.Second))
	var wsRawResponse string
	for attempt := 0; attempt < 5; attempt++ {
		if err := websocket.Message.Receive(wsConn, &wsRawResponse); err != nil {
			t.Fatalf("receive websocket message: %v", err)
		}
		var rpcResponse protocol.JSONRPCResponse
		if err := json.Unmarshal([]byte(wsRawResponse), &rpcResponse); err == nil && len(rpcResponse.JSONRPC) > 0 {
			if rpcResponse.Error != nil {
				t.Fatalf("unexpected websocket rpc error: %+v", rpcResponse.Error)
			}
			resultFrame, err := decodeJSONRPCResultFrame(rpcResponse)
			if err != nil {
				t.Fatalf("decode websocket result frame: %v", err)
			}
			if resultFrame.Action != FrameActionPing {
				t.Fatalf("websocket action = %q, want %q", resultFrame.Action, FrameActionPing)
			}
			break
		}
	}

	sseResponse, err := http.Get("http://" + listenAddress + "/sse?method=gateway.ping&id=sse-1")
	if err != nil {
		t.Fatalf("get /sse: %v", err)
	}
	defer sseResponse.Body.Close()
	if sseResponse.StatusCode != http.StatusOK {
		t.Fatalf("sse status = %d, want %d", sseResponse.StatusCode, http.StatusOK)
	}

	sseReader := bufio.NewReader(sseResponse.Body)
	resultFound := false
	currentEvent := ""
	timeout := time.After(3 * time.Second)
	for !resultFound {
		select {
		case <-timeout:
			t.Fatal("timed out waiting for sse result event")
		default:
			line, readErr := sseReader.ReadString('\n')
			if readErr != nil {
				t.Fatalf("read sse line: %v", readErr)
			}
			trimmedLine := strings.TrimSpace(line)
			if trimmedLine == "" {
				continue
			}
			if strings.HasPrefix(trimmedLine, "event:") {
				currentEvent = strings.TrimSpace(strings.TrimPrefix(trimmedLine, "event:"))
				continue
			}
			if strings.HasPrefix(trimmedLine, "data:") && currentEvent == "result" {
				rawData := strings.TrimSpace(strings.TrimPrefix(trimmedLine, "data:"))
				var rpcResponse protocol.JSONRPCResponse
				if err := json.Unmarshal([]byte(rawData), &rpcResponse); err != nil {
					t.Fatalf("decode sse result: %v", err)
				}
				if rpcResponse.Error != nil {
					t.Fatalf("unexpected sse rpc error: %+v", rpcResponse.Error)
				}
				resultFrame, err := decodeJSONRPCResultFrame(rpcResponse)
				if err != nil {
					t.Fatalf("decode sse result frame: %v", err)
				}
				if resultFrame.Action != FrameActionPing {
					t.Fatalf("sse action = %q, want %q", resultFrame.Action, FrameActionPing)
				}
				resultFound = true
			}
		}
	}
}

func TestNetworkServerCloseInterruptsStreams(t *testing.T) {
	server := newTestNetworkServer(t)
	testContext, cancel := context.WithCancel(context.Background())
	defer cancel()

	serveDone := make(chan error, 1)
	go func() {
		serveDone <- server.Serve(testContext, nil)
	}()
	listenAddress := waitForNetworkAddress(t, server)

	wsURL := "ws://" + listenAddress + "/ws"
	wsConn, err := websocket.Dial(wsURL, "", "http://localhost/")
	if err != nil {
		t.Fatalf("dial websocket: %v", err)
	}
	defer func() { _ = wsConn.Close() }()

	sseResponse, err := http.Get("http://" + listenAddress + "/sse?method=gateway.ping&id=sse-close")
	if err != nil {
		t.Fatalf("open sse stream: %v", err)
	}
	defer sseResponse.Body.Close()

	closeContext, closeCancel := context.WithTimeout(context.Background(), time.Second)
	defer closeCancel()
	if err := server.Close(closeContext); err != nil {
		t.Fatalf("close network server: %v", err)
	}

	_ = wsConn.SetReadDeadline(time.Now().Add(300 * time.Millisecond))
	var wsRawMessage string
	if err := websocket.Message.Receive(wsConn, &wsRawMessage); err == nil {
		t.Fatal("expected websocket receive to fail after server close")
	}

	readDone := make(chan error, 1)
	go func() {
		_, readErr := io.Copy(io.Discard, sseResponse.Body)
		readDone <- readErr
	}()

	select {
	case <-readDone:
	case <-time.After(time.Second):
		t.Fatal("sse stream was not closed after network server close")
	}

	select {
	case serveErr := <-serveDone:
		if serveErr != nil {
			t.Fatalf("serve returned error: %v", serveErr)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("serve did not exit after close")
	}
}

// newTestNetworkServer 创建默认测试网络服务实例，统一收敛测试参数。
func newTestNetworkServer(t *testing.T) *NetworkServer {
	t.Helper()

	server, err := NewNetworkServer(NetworkServerOptions{
		ListenAddress:     "127.0.0.1:0",
		Logger:            log.New(io.Discard, "", 0),
		HeartbeatInterval: 100 * time.Millisecond,
		ReadTimeout:       2 * time.Second,
		WriteTimeout:      2 * time.Second,
		ShutdownTimeout:   500 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("new network server: %v", err)
	}
	return server
}

// waitForNetworkAddress 等待网络服务绑定实际端口，避免使用 127.0.0.1:0 发起请求。
func waitForNetworkAddress(t *testing.T, server *NetworkServer) string {
	t.Helper()

	timeout := time.After(2 * time.Second)
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-timeout:
			t.Fatal("timed out waiting for network listen address")
		case <-ticker.C:
			address := server.ListenAddress()
			if !strings.HasSuffix(address, ":0") && strings.TrimSpace(address) != "" {
				return address
			}
		}
	}
}
