package core

import (
	"encoding/binary"
	"io"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/jossecurity/joss/pkg/parser"
)

func TestNativeWebSocketRouteStaysOpenAndSendsFrames(t *testing.T) {
	runtime := NewRuntime()
	defer runtime.Free()
	program := parser.NewParser(parser.NewLexer(`
public class NativeWebSocketProbe {
    public func connect(mixed $socket): void {
        $socket->send("connected")
    }
}`)).ParseProgram()
	class, ok := program.Statements[0].(*parser.ClassStatement)
	if !ok {
		t.Fatal("could not parse websocket probe class")
	}
	runtime.registerClass(class)
	runtime.Routes["WS"] = map[string]interface{}{
		"/ws/probe": map[string]interface{}{"handler": "NativeWebSocketProbe@connect"},
	}
	serverConn, clientConn := net.Pipe()
	defer clientConn.Close()
	dispatched, handled := runtime.callBuiltinIO("__joss_http_dispatch", []interface{}{
		"__websocket__", "/ws/probe", &Instance{Fields: map[string]interface{}{"_conn": serverConn}},
	})
	if !handled || dispatched != true {
		t.Fatal("native websocket dispatch refused a valid socket")
	}
	_ = clientConn.SetDeadline(time.Now().Add(2 * time.Second))
	var header [2]byte
	if _, err := io.ReadFull(clientConn, header[:]); err != nil {
		t.Fatalf("websocket closed before first server frame: %v", err)
	}
	if header[0] != 0x81 || header[1] != uint8(len("connected")) {
		t.Fatalf("unexpected server frame header: %v", header)
	}
	payload := make([]byte, int(header[1]&0x7f))
	if _, err := io.ReadFull(clientConn, payload); err != nil {
		t.Fatal(err)
	}
	if string(payload) != "connected" {
		t.Fatalf("server frame payload = %q, want connected", payload)
	}
	if _, err := clientConn.Write(maskedWebSocketFrame(0x8, nil)); err != nil {
		t.Fatalf("send close frame: %v", err)
	}
}

func TestNativeWebSocketReaderUnmasksText(t *testing.T) {
	serverConn, clientConn := net.Pipe()
	defer serverConn.Close()
	defer clientConn.Close()
	writeDone := make(chan error, 1)
	go func() { _, err := clientConn.Write(maskedWebSocketFrame(0x1, []byte("hello"))); writeDone <- err }()
	var writeMu sync.Mutex
	opcode, message, err := readNativeWebSocketMessage(serverConn, &writeMu, time.Second, defaultNativeWebSocketMessage)
	if err != nil {
		t.Fatal(err)
	}
	if opcode != 0x1 || string(message) != "hello" {
		t.Fatalf("decoded websocket message opcode=%d payload=%q", opcode, message)
	}
	if err := <-writeDone; err != nil {
		t.Fatal(err)
	}
}

func TestNativeWebSocketSendsKeepalivePing(t *testing.T) {
	runtime := NewRuntime()
	defer runtime.Free()
	runtime.Env["WS_PING_INTERVAL_SECONDS"] = "1"
	program := parser.NewParser(parser.NewLexer(`
public class NativeWebSocketIdleProbe {
    public func connect(mixed $socket): void { }
}`)).ParseProgram()
	class, ok := program.Statements[0].(*parser.ClassStatement)
	if !ok {
		t.Fatal("could not parse websocket idle probe class")
	}
	runtime.registerClass(class)
	runtime.Routes["WS"] = map[string]interface{}{
		"/ws/idle": map[string]interface{}{"handler": "NativeWebSocketIdleProbe@connect"},
	}
	serverConn, clientConn := net.Pipe()
	defer clientConn.Close()
	if !runtime.startNativeWebSocket("/ws/idle", &Instance{Fields: map[string]interface{}{"_conn": serverConn}}) {
		t.Fatal("native websocket dispatch refused a valid socket")
	}
	_ = clientConn.SetDeadline(time.Now().Add(2 * time.Second))
	var header [2]byte
	if _, err := io.ReadFull(clientConn, header[:]); err != nil {
		t.Fatalf("server did not send keepalive ping: %v", err)
	}
	if header[0] != 0x89 || header[1] != 0 {
		t.Fatalf("unexpected keepalive frame: %v", header)
	}
	if _, err := clientConn.Write(maskedWebSocketFrame(0x8, nil)); err != nil {
		t.Fatalf("send close frame: %v", err)
	}
}

func maskedWebSocketFrame(opcode byte, payload []byte) []byte {
	mask := [4]byte{0x12, 0x34, 0x56, 0x78}
	frame := []byte{0x80 | opcode}
	if len(payload) < 126 {
		frame = append(frame, 0x80|byte(len(payload)))
	} else {
		frame = append(frame, 0x80|126, byte(len(payload)>>8), byte(len(payload)))
	}
	frame = append(frame, mask[:]...)
	for i, value := range payload {
		frame = append(frame, value^mask[i&3])
	}
	return frame
}

func TestNativeWebSocketFrameWriterUsesCorrectExtendedLength(t *testing.T) {
	serverConn, clientConn := net.Pipe()
	defer serverConn.Close()
	defer clientConn.Close()
	payload := make([]byte, 300)
	writeDone := make(chan error, 1)
	go func() { writeDone <- writeNativeWebSocketFrame(serverConn, 0x1, payload) }()
	var header [4]byte
	if _, err := io.ReadFull(clientConn, header[:]); err != nil {
		t.Fatal(err)
	}
	if header[0] != 0x81 || header[1] != 126 || binary.BigEndian.Uint16(header[2:]) != 300 {
		t.Fatalf("unexpected extended frame header: %v", header)
	}
	if _, err := io.CopyN(io.Discard, clientConn, int64(len(payload))); err != nil {
		t.Fatal(err)
	}
	if err := <-writeDone; err != nil {
		t.Fatal(err)
	}
}
