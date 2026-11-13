package core

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"strconv"
	"sync"
	"time"
)

const defaultNativeWebSocketMessage = 8 << 20
const maxNativeWebSocketMessage = 1 << 30

// startNativeWebSocket hands a raw upgraded socket to an isolated request runtime.
// Joss owns the route and callback semantics; Go provides the byte-oriented frame I/O.
func (r *Runtime) startNativeWebSocket(path string, socket *Instance) bool {
	if socket == nil {
		return false
	}
	conn, ok := socket.Fields["_conn"].(net.Conn)
	if !ok || conn == nil {
		return false
	}
	requestRuntime := r.Fork()
	if requestRuntime == nil {
		_ = conn.Close()
		return false
	}
	go func() {
		defer requestRuntime.Free()
		defer conn.Close()
		var writeMu sync.Mutex
		idleTimeout := websocketDuration(requestRuntime.Env["WS_IDLE_TIMEOUT_SECONDS"], 120*time.Second)
		pingInterval := websocketDuration(requestRuntime.Env["WS_PING_INTERVAL_SECONDS"], 30*time.Second)
		messageLimit := websocketMessageLimit(requestRuntime.Env["WS_MAX_MESSAGE_BYTES"])
		stopPing := make(chan struct{})
		pingDone := make(chan struct{})
		go func() {
			defer close(pingDone)
			ticker := time.NewTicker(pingInterval)
			defer ticker.Stop()
			for {
				select {
				case <-stopPing:
					return
				case <-ticker.C:
					writeMu.Lock()
					err := writeNativeWebSocketFrame(conn, 0x9, nil)
					writeMu.Unlock()
					if err != nil {
						_ = conn.Close()
						return
					}
				}
			}
		}()
		defer func() {
			close(stopPing)
			_ = conn.Close()
			<-pingDone
		}()
		reader := func() (int, []byte, error) {
			return readNativeWebSocketMessage(conn, &writeMu, idleTimeout, messageLimit)
		}
		sender := func(value interface{}) error {
			payload, ok := value.(string)
			if !ok {
				encoded, err := json.Marshal(value)
				if err != nil {
					return fmt.Errorf("encode websocket payload: %w", err)
				}
				payload = string(encoded)
			}
			if uint64(len(payload)) > messageLimit {
				return fmt.Errorf("websocket message exceeds %d bytes", messageLimit)
			}
			writeMu.Lock()
			defer writeMu.Unlock()
			return writeNativeWebSocketFrame(conn, 0x1, []byte(payload))
		}
		closer := func() error { return conn.Close() }
		requestRuntime.DispatchWebSocket(path, conn, reader, sender, closer)
	}()
	return true
}

func websocketDuration(configured string, fallback time.Duration) time.Duration {
	seconds, err := strconv.Atoi(configured)
	if err != nil || seconds <= 0 {
		return fallback
	}
	return time.Duration(seconds) * time.Second
}

func websocketMessageLimit(configured string) uint64 {
	limit, err := strconv.ParseUint(configured, 10, 64)
	if err != nil || limit == 0 {
		return defaultNativeWebSocketMessage
	}
	if limit > maxNativeWebSocketMessage {
		return maxNativeWebSocketMessage
	}
	return limit
}

func readNativeWebSocketMessage(conn net.Conn, writeMu *sync.Mutex, idleTimeout time.Duration, messageLimit uint64) (int, []byte, error) {
	var message []byte
	messageOpcode := 0
	for {
		if idleTimeout > 0 {
			_ = conn.SetReadDeadline(time.Now().Add(idleTimeout))
		}
		var header [2]byte
		if _, err := io.ReadFull(conn, header[:]); err != nil {
			return 0, nil, err
		}
		fin := header[0]&0x80 != 0
		opcode := int(header[0] & 0x0f)
		masked := header[1]&0x80 != 0
		payloadLength := uint64(header[1] & 0x7f)
		if payloadLength == 126 {
			var extended [2]byte
			if _, err := io.ReadFull(conn, extended[:]); err != nil {
				return 0, nil, err
			}
			payloadLength = uint64(binary.BigEndian.Uint16(extended[:]))
		} else if payloadLength == 127 {
			var extended [8]byte
			if _, err := io.ReadFull(conn, extended[:]); err != nil {
				return 0, nil, err
			}
			payloadLength = binary.BigEndian.Uint64(extended[:])
		}
		if payloadLength > messageLimit || (opcode >= 0x8 && (!fin || payloadLength > 125)) {
			return 0, nil, fmt.Errorf("invalid or oversized websocket frame")
		}
		var mask [4]byte
		if masked {
			if _, err := io.ReadFull(conn, mask[:]); err != nil {
				return 0, nil, err
			}
		}
		payload := make([]byte, int(payloadLength))
		if _, err := io.ReadFull(conn, payload); err != nil {
			return 0, nil, err
		}
		if masked {
			for i := range payload {
				payload[i] ^= mask[i&3]
			}
		}

		switch opcode {
		case 0x8:
			writeMu.Lock()
			_ = writeNativeWebSocketFrame(conn, 0x8, payload)
			writeMu.Unlock()
			return 0, nil, io.EOF
		case 0x9:
			writeMu.Lock()
			err := writeNativeWebSocketFrame(conn, 0xA, payload)
			writeMu.Unlock()
			if err != nil {
				return 0, nil, err
			}
			continue
		case 0xA:
			continue
		case 0x1, 0x2:
			if messageOpcode != 0 {
				return 0, nil, fmt.Errorf("nested websocket data frame")
			}
			messageOpcode = opcode
			message = append(message, payload...)
		case 0x0:
			if messageOpcode == 0 {
				return 0, nil, fmt.Errorf("unexpected websocket continuation frame")
			}
			message = append(message, payload...)
		default:
			return 0, nil, fmt.Errorf("unsupported websocket opcode %d", opcode)
		}
		if uint64(len(message)) > messageLimit {
			return 0, nil, fmt.Errorf("oversized websocket message")
		}
		if fin {
			return messageOpcode, message, nil
		}
	}
}

func writeNativeWebSocketFrame(conn net.Conn, opcode byte, payload []byte) error {
	frame := make([]byte, 0, len(payload)+10)
	frame = append(frame, 0x80|(opcode&0x0f))
	switch n := len(payload); {
	case n < 126:
		frame = append(frame, byte(n))
	case n <= 65535:
		frame = append(frame, 126, byte(n>>8), byte(n))
	default:
		frame = append(frame, 127)
		var extended [8]byte
		binary.BigEndian.PutUint64(extended[:], uint64(n))
		frame = append(frame, extended[:]...)
	}
	frame = append(frame, payload...)
	for len(frame) > 0 {
		n, err := conn.Write(frame)
		if err != nil {
			return err
		}
		if n == 0 {
			return io.ErrShortWrite
		}
		frame = frame[n:]
	}
	return nil
}
