package core

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"time"
)

func (r *Runtime) socketContext() context.Context {
	if r.executionContext != nil {
		return r.executionContext
	}
	return context.Background()
}

func (r *Runtime) socketDeadline() (time.Time, bool) {
	if r.executionContext == nil {
		return time.Time{}, false
	}
	return r.executionContext.Deadline()
}

func (r *Runtime) checkSocketCancellation(err error) {
	if r.executionContext == nil {
		return
	}
	var networkError net.Error
	if errors.As(err, &networkError) && networkError.Timeout() {
		if _, hasDeadline := r.executionContext.Deadline(); hasDeadline {
			<-r.executionContext.Done()
		}
	}
	r.checkExecutionCancelled()
}

// executeSocketMethod handles methods on Socket instances and static calls
func (r *Runtime) executeSocketMethod(instance *Instance, method string, args []interface{}) interface{} {
	if method != "close" {
		r.checkExecutionCancelled()
	}
	if method != "close" && method != "port" {
		if err := r.RequireCapability("network"); err != nil {
			panic(err)
		}
	}
	switch method {
	case "tcp", "connect":
		host := "127.0.0.1"
		port := "80"
		if len(args) > 0 {
			host = fmt.Sprintf("%v", args[0])
		}
		if len(args) > 1 {
			port = fmt.Sprintf("%v", args[1])
		}
		addr := net.JoinHostPort(host, port)
		conn, err := (&net.Dialer{}).DialContext(r.socketContext(), "tcp", addr)
		if err != nil {
			r.checkSocketCancellation(err)
			panic(&JossError{Type: "SocketError", Message: fmt.Sprintf("Socket::connect failed: %v", err), File: r.CurrentFile})
		}
		inst := instance
		if inst == nil || inst.Class == nil || inst.Class.Name == nil || inst.Class.Name.Value != "Socket" {
			inst = &Instance{Class: r.Classes["Socket"], Fields: make(map[string]interface{})}
		}
		inst.Fields["_conn"] = conn
		return inst

	case "listen":
		host := "127.0.0.1"
		port := "0"
		if len(args) > 0 {
			host = fmt.Sprintf("%v", args[0])
		}
		if len(args) > 1 {
			port = fmt.Sprintf("%v", args[1])
		}
		addr := net.JoinHostPort(host, port)
		listener, err := net.Listen("tcp", addr)
		if err != nil {
			panic(&JossError{Type: "SocketError", Message: fmt.Sprintf("Socket::listen failed: %v", err), File: r.CurrentFile})
		}
		inst := instance
		if inst == nil || inst.Class == nil || inst.Class.Name == nil || inst.Class.Name.Value != "Socket" {
			inst = &Instance{Class: r.Classes["Socket"], Fields: make(map[string]interface{})}
		}
		inst.Fields["_listener"] = listener
		inst.Fields["_port"] = int64(listener.Addr().(*net.TCPAddr).Port)
		return inst

	case "accept":
		listener, ok := instance.Fields["_listener"].(net.Listener)
		if !ok || listener == nil {
			panic(&JossError{Type: "SocketError", Message: "Socket is not listening", File: r.CurrentFile})
		}
		if deadline, hasDeadline := r.socketDeadline(); hasDeadline {
			if tcpListener, ok := listener.(*net.TCPListener); ok {
				_ = tcpListener.SetDeadline(deadline)
				defer tcpListener.SetDeadline(time.Time{})
			}
		}
		conn, err := listener.Accept()
		if err != nil {
			if r.executionContext != nil && r.executionContext.Err() != nil {
				_ = listener.Close()
				instance.Fields["_listener"] = nil
			}
			r.checkSocketCancellation(err)
			panic(&JossError{Type: "SocketError", Message: fmt.Sprintf("Socket::accept failed: %v", err), File: r.CurrentFile})
		}
		clientInst := &Instance{Class: r.Classes["Socket"], Fields: make(map[string]interface{})}
		clientInst.Fields["_conn"] = conn
		return clientInst

	case "send":
		conn, ok := instance.Fields["_conn"].(net.Conn)
		if !ok || conn == nil {
			panic(&JossError{Type: "SocketError", Message: "Socket is not connected", File: r.CurrentFile})
		}
		data := ""
		if len(args) > 0 {
			data = fmt.Sprintf("%v", args[0])
		}
		if deadline, hasDeadline := r.socketDeadline(); hasDeadline {
			_ = conn.SetWriteDeadline(deadline)
			defer conn.SetWriteDeadline(time.Time{})
		}
		n, err := conn.Write([]byte(data))
		if err != nil {
			r.checkSocketCancellation(err)
			panic(&JossError{Type: "SocketError", Message: fmt.Sprintf("Socket::send failed: %v", err), File: r.CurrentFile})
		}
		return int64(n)

	case "receive":
		conn, ok := instance.Fields["_conn"].(net.Conn)
		if !ok || conn == nil {
			panic(&JossError{Type: "SocketError", Message: "Socket is not connected", File: r.CurrentFile})
		}
		maxBytes := 4096
		if len(args) > 0 {
			if n, ok := args[0].(int64); ok && n > 0 {
				maxBytes = int(n)
			} else if n, ok := args[0].(int); ok && n > 0 {
				maxBytes = n
			} else if s, ok := args[0].(string); ok {
				if parsed, err := strconv.Atoi(s); err == nil && parsed > 0 {
					maxBytes = parsed
				}
			}
		}
		if deadline, hasDeadline := r.socketDeadline(); hasDeadline {
			_ = conn.SetReadDeadline(deadline)
			defer conn.SetReadDeadline(time.Time{})
		}
		buf := make([]byte, maxBytes)
		n, err := conn.Read(buf)
		if err != nil && err != io.EOF {
			r.checkSocketCancellation(err)
			panic(&JossError{Type: "SocketError", Message: fmt.Sprintf("Socket::receive failed: %v", err), File: r.CurrentFile})
		}
		return string(buf[:n])

	case "port":
		if p, ok := instance.Fields["_port"].(int64); ok {
			return p
		}
		if listener, ok := instance.Fields["_listener"].(net.Listener); ok && listener != nil {
			if tcpAddr, ok := listener.Addr().(*net.TCPAddr); ok {
				return int64(tcpAddr.Port)
			}
		}
		return int64(0)

	case "close":
		if conn, ok := instance.Fields["_conn"].(net.Conn); ok && conn != nil {
			_ = conn.Close()
			instance.Fields["_conn"] = nil
		}
		if listener, ok := instance.Fields["_listener"].(net.Listener); ok && listener != nil {
			_ = listener.Close()
			instance.Fields["_listener"] = nil
		}
		return nil
	}
	return nil
}
